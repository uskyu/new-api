package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCheckinCaptchaSingleUse(t *testing.T) {
	require.NoError(t, (checkinCaptchaStore{}).Set("c1", "42"))
	require.NoError(t, VerifyCheckinCaptcha("c1", "42"))
	require.Error(t, VerifyCheckinCaptcha("c1", "42"))
}

func TestCheckinCaptchaWrongAnswerAllowsRetry(t *testing.T) {
	require.NoError(t, (checkinCaptchaStore{}).Set("c2", "42"))
	require.Error(t, VerifyCheckinCaptcha("c2", "43"))
	require.NoError(t, VerifyCheckinCaptcha("c2", "42"))
}

func TestCheckinCaptchaTooManyAttempts(t *testing.T) {
	require.NoError(t, (checkinCaptchaStore{}).Set("c3", "42"))
	for i := 0; i < checkinCaptchaMaxAttempts; i++ {
		require.Error(t, VerifyCheckinCaptcha("c3", "43"))
	}
	require.Error(t, VerifyCheckinCaptcha("c3", "42"))
}

func TestCheckinCaptchaExpired(t *testing.T) {
	checkinCaptchaMu.Lock()
	checkinCaptchaChallenges["c4"] = &checkinCaptchaChallenge{
		Answer:   "42",
		ExpireAt: time.Now().Add(-time.Minute),
	}
	checkinCaptchaMu.Unlock()
	require.Error(t, VerifyCheckinCaptcha("c4", "42"))
}
