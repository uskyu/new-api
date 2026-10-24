package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestGetUniqueTokenByNameRejectsAmbiguousName protects the Harness service
// subject mapping: token names are only indexed, not unique, so two rows sharing
// a name must fail loudly instead of binding the platform subject to whichever
// row the planner returned first.
func TestGetUniqueTokenByNameRejectsAmbiguousName(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Token{}))
	t.Cleanup(func() { DB.Exec("DELETE FROM tokens WHERE name LIKE 'harness-subject-%'") })

	unique := &Token{Name: "harness-subject-unique", Key: "harness-subject-key-unique-1", UserId: 1}
	require.NoError(t, DB.Create(unique).Error)

	resolved, err := GetUniqueTokenByName("harness-subject-unique")
	require.NoError(t, err)
	assert.Equal(t, unique.Id, resolved.Id)

	_, err = GetUniqueTokenByName("harness-subject-absent")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	require.NoError(t, DB.Create(&Token{Name: "harness-subject-dup", Key: "harness-subject-key-dup-1", UserId: 1}).Error)
	require.NoError(t, DB.Create(&Token{Name: "harness-subject-dup", Key: "harness-subject-key-dup-2", UserId: 2}).Error)

	_, err = GetUniqueTokenByName("harness-subject-dup")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous")

	_, err = GetUniqueTokenByName("")
	require.Error(t, err)
}
