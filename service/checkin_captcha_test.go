package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func seedImageCaptcha(t *testing.T, state *imageCaptchaState, id string, answer string) {
	t.Helper()
	require.NoError(t, imageCaptchaStore{state: state}.Set(id, answer))
}

func TestCheckinCaptchaSingleUse(t *testing.T) {
	seedImageCaptcha(t, checkinCaptcha, "c1", "42")
	require.NoError(t, VerifyCheckinCaptcha("c1", "42"))
	require.Error(t, VerifyCheckinCaptcha("c1", "42"))
}

func TestCheckinCaptchaWrongAnswerAllowsRetry(t *testing.T) {
	seedImageCaptcha(t, checkinCaptcha, "c2", "42")
	require.Error(t, VerifyCheckinCaptcha("c2", "43"))
	require.NoError(t, VerifyCheckinCaptcha("c2", "42"))
}

func TestCheckinCaptchaTooManyAttempts(t *testing.T) {
	seedImageCaptcha(t, checkinCaptcha, "c3", "42")
	for range imageCaptchaMaxAttempts {
		require.Error(t, VerifyCheckinCaptcha("c3", "43"))
	}
	require.Error(t, VerifyCheckinCaptcha("c3", "42"))
}

func TestCheckinCaptchaExpired(t *testing.T) {
	checkinCaptcha.mu.Lock()
	checkinCaptcha.challenges["c4"] = &imageCaptchaChallenge{
		Answer:   "42",
		ExpireAt: time.Now().Add(-time.Minute),
	}
	checkinCaptcha.mu.Unlock()
	require.Error(t, VerifyCheckinCaptcha("c4", "42"))
}

func TestRegisterCaptchaSingleUse(t *testing.T) {
	seedImageCaptcha(t, registerCaptcha, "r1", "42")
	require.NoError(t, VerifyRegisterCaptcha("r1", "42"))
	require.Error(t, VerifyRegisterCaptcha("r1", "42"))
}

func TestRegisterCaptchaEmptyParams(t *testing.T) {
	require.Error(t, VerifyRegisterCaptcha("", ""))
	seedImageCaptcha(t, registerCaptcha, "r2", "42")
	require.Error(t, VerifyRegisterCaptcha("r2", ""))
}

// 注册与签到的验证码挑战必须互相隔离，不能用一个入口的验证码通过另一个入口。
func TestRegisterCaptchaPurposeIsolation(t *testing.T) {
	seedImageCaptcha(t, registerCaptcha, "iso-register", "42")
	require.Error(t, VerifyCheckinCaptcha("iso-register", "42"))
	require.NoError(t, VerifyRegisterCaptcha("iso-register", "42"))

	seedImageCaptcha(t, checkinCaptcha, "iso-checkin", "42")
	require.Error(t, VerifyRegisterCaptcha("iso-checkin", "42"))
	require.NoError(t, VerifyCheckinCaptcha("iso-checkin", "42"))
}

func TestRegisterCaptchaGenerate(t *testing.T) {
	id, imageBase64, err := GenerateRegisterCaptcha("digit")
	require.NoError(t, err)
	require.NotEmpty(t, id)
	require.Contains(t, imageBase64, "base64")
}
