package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTwoFATestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := DB, LOG_DB
	db := openTestDB(t, TestDBDialectSQLite)
	DB, LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&TwoFA{}))
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		DB, LOG_DB = previousDB, previousLogDB
	})
	return db
}

func TestIncrementFailedAttemptsRestartsWindowAfterLockoutExpires(t *testing.T) {
	for _, tc := range []struct {
		name            string
		lockedUntil     time.Duration
		failures        int
		wantAttempts    int
		wantLocked      bool
		keepLockedUntil bool
	}{
		{name: "first failure after expiry", lockedUntil: -time.Minute, failures: 1, wantAttempts: 1, wantLocked: false},
		{name: "max failures after expiry", lockedUntil: -time.Minute, failures: common.MaxFailAttempts, wantAttempts: common.MaxFailAttempts, wantLocked: true},
		{name: "failure during active lockout", lockedUntil: time.Minute, failures: 1, wantAttempts: common.MaxFailAttempts, wantLocked: true, keepLockedUntil: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupTwoFATestDB(t)

			lockedUntil := time.Now().Add(tc.lockedUntil)
			twoFA := TwoFA{UserId: 42, Secret: "secret", IsEnabled: true, FailedAttempts: common.MaxFailAttempts, LockedUntil: &lockedUntil}
			require.NoError(t, DB.Create(&twoFA).Error)
			var stored TwoFA
			require.NoError(t, DB.First(&stored, twoFA.Id).Error)

			for range tc.failures {
				loaded, err := GetTwoFAByUserId(twoFA.UserId)
				require.NoError(t, err)
				require.NotNil(t, loaded)
				require.NoError(t, loaded.IncrementFailedAttempts())
			}

			var reloaded TwoFA
			require.NoError(t, DB.First(&reloaded, twoFA.Id).Error)
			assert.Equal(t, tc.wantAttempts, reloaded.FailedAttempts)
			assert.Equal(t, tc.wantLocked, reloaded.IsLocked())
			if !tc.wantLocked {
				assert.Nil(t, reloaded.LockedUntil)
			}
			if tc.keepLockedUntil {
				require.NotNil(t, reloaded.LockedUntil)
				assert.True(t, stored.LockedUntil.Equal(*reloaded.LockedUntil), "lockout must not be extended")
			}
		})
	}
}
