package service

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/mojocn/base64Captcha"
)

const (
	checkinCaptchaTTL          = 5 * time.Minute
	checkinCaptchaMaxAttempts  = 5
	checkinCaptchaExpiredError = "验证码不存在或已过期"
	checkinCaptchaEmptyError   = "验证码参数为空"
	checkinCaptchaWrongError   = "验证码错误，请重新输入"
	checkinCaptchaTooManyError = "验证码错误次数过多，请重新获取"
)

type checkinCaptchaChallenge struct {
	Answer   string
	ExpireAt time.Time
	Attempts int
}

var (
	checkinCaptchaMu         sync.Mutex
	checkinCaptchaChallenges = make(map[string]*checkinCaptchaChallenge)
)

type checkinCaptchaStore struct{}

func (checkinCaptchaStore) Set(id string, value string) error {
	checkinCaptchaMu.Lock()
	defer checkinCaptchaMu.Unlock()
	challenge := &checkinCaptchaChallenge{
		Answer:   value,
		ExpireAt: time.Now().Add(checkinCaptchaTTL),
	}
	checkinCaptchaChallenges[id] = challenge
	time.AfterFunc(checkinCaptchaTTL, func() {
		checkinCaptchaMu.Lock()
		defer checkinCaptchaMu.Unlock()
		if current, ok := checkinCaptchaChallenges[id]; ok && current == challenge {
			delete(checkinCaptchaChallenges, id)
		}
	})
	return nil
}

func (checkinCaptchaStore) Get(id string, clear bool) string {
	checkinCaptchaMu.Lock()
	defer checkinCaptchaMu.Unlock()
	challenge, ok := checkinCaptchaChallenges[id]
	if !ok || time.Now().After(challenge.ExpireAt) {
		delete(checkinCaptchaChallenges, id)
		return ""
	}
	answer := challenge.Answer
	if clear {
		delete(checkinCaptchaChallenges, id)
	}
	return answer
}

func (checkinCaptchaStore) Verify(id string, answer string, clear bool) bool {
	return VerifyCheckinCaptcha(id, answer) == nil
}

// GenerateCheckinCaptcha creates a self-hosted image captcha challenge.
func GenerateCheckinCaptcha(kind string) (id string, imageBase64 string, err error) {
	var driver base64Captcha.Driver
	switch kind {
	case "digit":
		driver = base64Captcha.NewDriverDigit(80, 240, 4, 0.7, 80)
	default:
		driver = base64Captcha.NewDriverMath(
			80,
			240,
			6,
			base64Captcha.OptionShowSineLine|base64Captcha.OptionShowSlimeLine,
			nil,
			nil,
			nil,
		)
	}
	captcha := base64Captcha.NewCaptcha(driver, checkinCaptchaStore{})
	id, imageBase64, _, err = captcha.Generate()
	if err != nil {
		return "", "", err
	}
	return id, imageBase64, nil
}

// VerifyCheckinCaptcha checks a one-time challenge and consumes it on success.
func VerifyCheckinCaptcha(id string, answer string) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(answer) == "" {
		return errors.New(checkinCaptchaEmptyError)
	}

	checkinCaptchaMu.Lock()
	defer checkinCaptchaMu.Unlock()

	challenge, ok := checkinCaptchaChallenges[id]
	if !ok || time.Now().After(challenge.ExpireAt) {
		delete(checkinCaptchaChallenges, id)
		return errors.New(checkinCaptchaExpiredError)
	}
	if challenge.Attempts >= checkinCaptchaMaxAttempts {
		delete(checkinCaptchaChallenges, id)
		return errors.New(checkinCaptchaTooManyError)
	}
	if !strings.EqualFold(strings.TrimSpace(challenge.Answer), strings.TrimSpace(answer)) {
		challenge.Attempts++
		if challenge.Attempts >= checkinCaptchaMaxAttempts {
			delete(checkinCaptchaChallenges, id)
		}
		return errors.New(checkinCaptchaWrongError)
	}
	delete(checkinCaptchaChallenges, id)
	return nil
}
