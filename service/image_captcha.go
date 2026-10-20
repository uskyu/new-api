package service

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/mojocn/base64Captcha"
)

const (
	imageCaptchaTTL          = 5 * time.Minute
	imageCaptchaMaxAttempts  = 5
	imageCaptchaExpiredError = "验证码不存在或已过期"
	imageCaptchaEmptyError   = "验证码参数为空"
	imageCaptchaWrongError   = "验证码错误，请重新输入"
	imageCaptchaTooManyError = "验证码错误次数过多，请重新获取"
)

// imageCaptchaChallenge 是一次性图形验证码挑战。
type imageCaptchaChallenge struct {
	Answer   string
	ExpireAt time.Time
	Attempts int
}

// imageCaptchaState 保存某一用途（签到/注册）的全部验证码挑战。
// 使用进程内存储即可满足单实例部署；多实例部署需要会话粘滞或改用共享存储。
type imageCaptchaState struct {
	mu         sync.Mutex
	challenges map[string]*imageCaptchaChallenge
}

func newImageCaptchaState() *imageCaptchaState {
	return &imageCaptchaState{
		challenges: make(map[string]*imageCaptchaChallenge),
	}
}

// imageCaptchaStore 实现 base64Captcha.Store，仅用于生成验证码时写入答案。
type imageCaptchaStore struct {
	state *imageCaptchaState
}

func (s imageCaptchaStore) Set(id string, value string) error {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	challenge := &imageCaptchaChallenge{
		Answer:   value,
		ExpireAt: time.Now().Add(imageCaptchaTTL),
	}
	s.state.challenges[id] = challenge
	time.AfterFunc(imageCaptchaTTL, func() {
		s.state.mu.Lock()
		defer s.state.mu.Unlock()
		if current, ok := s.state.challenges[id]; ok && current == challenge {
			delete(s.state.challenges, id)
		}
	})
	return nil
}

func (s imageCaptchaStore) Get(id string, clear bool) string {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	challenge, ok := s.state.challenges[id]
	if !ok || time.Now().After(challenge.ExpireAt) {
		delete(s.state.challenges, id)
		return ""
	}
	answer := challenge.Answer
	if clear {
		delete(s.state.challenges, id)
	}
	return answer
}

func (s imageCaptchaStore) Verify(id string, answer string, clear bool) bool {
	return s.state.verify(id, answer) == nil
}

// generate 生成一张自托管图形验证码，kind 支持 math / digit。
func (s *imageCaptchaState) generate(kind string) (id string, imageBase64 string, err error) {
	captcha := base64Captcha.NewCaptcha(imageCaptchaDriver(kind), imageCaptchaStore{state: s})
	id, imageBase64, _, err = captcha.Generate()
	if err != nil {
		return "", "", err
	}
	return id, imageBase64, nil
}

func imageCaptchaDriver(kind string) base64Captcha.Driver {
	if kind == "digit" {
		return base64Captcha.NewDriverDigit(80, 240, 4, 0.7, 80)
	}
	return base64Captcha.NewDriverMath(
		80,
		240,
		6,
		base64Captcha.OptionShowSineLine|base64Captcha.OptionShowSlimeLine,
		nil,
		nil,
		nil,
	)
}

// verify 校验并消费一次性验证码，错误答案会累计尝试次数。
func (s *imageCaptchaState) verify(id string, answer string) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(answer) == "" {
		return errors.New(imageCaptchaEmptyError)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	challenge, ok := s.challenges[id]
	if !ok || time.Now().After(challenge.ExpireAt) {
		delete(s.challenges, id)
		return errors.New(imageCaptchaExpiredError)
	}
	if challenge.Attempts >= imageCaptchaMaxAttempts {
		delete(s.challenges, id)
		return errors.New(imageCaptchaTooManyError)
	}
	if !strings.EqualFold(strings.TrimSpace(challenge.Answer), strings.TrimSpace(answer)) {
		challenge.Attempts++
		if challenge.Attempts >= imageCaptchaMaxAttempts {
			delete(s.challenges, id)
		}
		return errors.New(imageCaptchaWrongError)
	}
	delete(s.challenges, id)
	return nil
}
