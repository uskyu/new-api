package controller

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestResolveRegistrationInviterId(t *testing.T) {
	t.Run("empty code skips lookup", func(t *testing.T) {
		called := false
		inviterId, err := resolveRegistrationInviterId("", func(string) (int, error) {
			called = true
			return 0, errors.New("unexpected lookup")
		})
		require.NoError(t, err)
		require.Zero(t, inviterId)
		require.False(t, called)
	})

	t.Run("record not found means no invitation", func(t *testing.T) {
		inviterId, err := resolveRegistrationInviterId("missing", func(string) (int, error) {
			return 0, gorm.ErrRecordNotFound
		})
		require.NoError(t, err)
		require.Zero(t, inviterId)
	})

	t.Run("database error is returned", func(t *testing.T) {
		databaseErr := errors.New("database unavailable")
		inviterId, err := resolveRegistrationInviterId("owner", func(string) (int, error) {
			return 0, databaseErr
		})
		require.ErrorIs(t, err, databaseErr)
		require.Zero(t, inviterId)
	})

	t.Run("valid inviter is returned", func(t *testing.T) {
		inviterId, err := resolveRegistrationInviterId("owner", func(string) (int, error) {
			return 42, nil
		})
		require.NoError(t, err)
		require.Equal(t, 42, inviterId)
	})
}
