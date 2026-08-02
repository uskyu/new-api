package model

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	AuthFlowPurposeQuickLogin       = "quick_login"
	AuthFlowTokenBytes              = 32
	AuthFlowDefaultCleanupRetention = 24 * time.Hour
)

var (
	ErrAuthFlowInvalid  = errors.New("auth flow is invalid")
	ErrAuthFlowExpired  = errors.New("auth flow has expired")
	ErrAuthFlowConsumed = errors.New("auth flow has already been consumed")
	ErrAuthFlowApproved = errors.New("auth flow has already been approved")
)

// AuthFlow stores short-lived, one-time authorization state. The opaque token
// is returned to the client once and only its HMAC is persisted.
type AuthFlow struct {
	Id         int64      `json:"id" gorm:"primaryKey"`
	TokenHash  string     `json:"-" gorm:"type:char(64);not null;uniqueIndex"`
	Purpose    string     `json:"purpose" gorm:"type:varchar(32);not null;index:idx_auth_flow_purpose_expiry"`
	Provider   string     `json:"provider,omitempty" gorm:"type:varchar(64)"`
	Intent     string     `json:"intent,omitempty" gorm:"type:varchar(16)"`
	UserId     int        `json:"user_id,omitempty" gorm:"index"`
	SessionId  string     `json:"session_id,omitempty" gorm:"type:varchar(64);index"`
	Payload    string     `json:"-" gorm:"type:text"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at" gorm:"not null;index:idx_auth_flow_purpose_expiry"`
	ConsumedAt *time.Time `json:"consumed_at,omitempty" gorm:"index"`
}

func (AuthFlow) TableName() string {
	return "auth_flows"
}

type AuthFlowCreate struct {
	Purpose   string
	Provider  string
	Intent    string
	UserId    int
	SessionId string
	Payload   string
	ExpiresAt time.Time
}

type AuthFlowMatch struct {
	Purpose   string
	Provider  string
	Intent    string
	UserId    int
	SessionId string
}

func applyAuthFlowMatch(query *gorm.DB, token string, match AuthFlowMatch) *gorm.DB {
	query = query.Where("token_hash = ? AND purpose = ?", authFlowTokenHash(token), match.Purpose)
	if match.Provider != "" {
		query = query.Where("provider = ?", match.Provider)
	}
	if match.Intent != "" {
		query = query.Where("intent = ?", match.Intent)
	}
	if match.UserId != 0 {
		query = query.Where("user_id = ?", match.UserId)
	}
	if match.SessionId != "" {
		query = query.Where("session_id = ?", match.SessionId)
	}
	return query
}

func authFlowTokenHash(token string) string {
	return common.GenerateHMACWithKey([]byte("auth-flow-v1:"+common.SessionSecret), token)
}

func CreateAuthFlow(input AuthFlowCreate) (string, *AuthFlow, error) {
	if strings.TrimSpace(input.Purpose) == "" || input.ExpiresAt.IsZero() || !input.ExpiresAt.After(time.Now()) {
		return "", nil, ErrAuthFlowInvalid
	}
	random := make([]byte, AuthFlowTokenBytes)
	if _, err := rand.Read(random); err != nil {
		return "", nil, fmt.Errorf("generate auth flow token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	flow := &AuthFlow{
		TokenHash: authFlowTokenHash(token),
		Purpose:   input.Purpose,
		Provider:  input.Provider,
		Intent:    input.Intent,
		UserId:    input.UserId,
		SessionId: input.SessionId,
		Payload:   input.Payload,
		ExpiresAt: input.ExpiresAt,
	}
	if err := DB.Create(flow).Error; err != nil {
		return "", nil, err
	}
	return token, flow, nil
}

func GetAuthFlow(token string, match AuthFlowMatch) (*AuthFlow, error) {
	if token == "" || match.Purpose == "" {
		return nil, ErrAuthFlowInvalid
	}
	var flow AuthFlow
	if err := applyAuthFlowMatch(DB, token, match).First(&flow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAuthFlowInvalid
		}
		return nil, err
	}
	if flow.ConsumedAt != nil {
		return nil, ErrAuthFlowConsumed
	}
	if !flow.ExpiresAt.After(time.Now()) {
		return nil, ErrAuthFlowExpired
	}
	return &flow, nil
}

func ApproveAuthFlow(token string, match AuthFlowMatch, userId int) (*AuthFlow, error) {
	if token == "" || match.Purpose == "" || userId <= 0 {
		return nil, ErrAuthFlowInvalid
	}
	var approved AuthFlow
	err := DB.Transaction(func(tx *gorm.DB) error {
		query := applyAuthFlowMatch(lockForUpdate(tx), token, match)
		if err := query.First(&approved).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAuthFlowInvalid
			}
			return err
		}
		if approved.ConsumedAt != nil {
			return ErrAuthFlowConsumed
		}
		if !approved.ExpiresAt.After(time.Now()) {
			return ErrAuthFlowExpired
		}
		if approved.UserId != 0 {
			if approved.UserId == userId {
				return nil
			}
			return ErrAuthFlowApproved
		}
		result := tx.Model(&AuthFlow{}).
			Where("id = ? AND user_id = 0 AND consumed_at IS NULL", approved.Id).
			Update("user_id", userId)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrAuthFlowApproved
		}
		approved.UserId = userId
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &approved, nil
}

func ConsumeAuthFlowWithAction(token string, match AuthFlowMatch, action func(tx *gorm.DB, flow *AuthFlow) error) (*AuthFlow, error) {
	if token == "" || match.Purpose == "" {
		return nil, ErrAuthFlowInvalid
	}
	var consumed AuthFlow
	err := DB.Transaction(func(tx *gorm.DB) error {
		query := applyAuthFlowMatch(lockForUpdate(tx), token, match)
		if err := query.First(&consumed).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAuthFlowInvalid
			}
			return err
		}
		if consumed.ConsumedAt != nil {
			return ErrAuthFlowConsumed
		}
		now := time.Now()
		if !consumed.ExpiresAt.After(now) {
			return ErrAuthFlowExpired
		}
		result := tx.Model(&AuthFlow{}).
			Where("id = ? AND consumed_at IS NULL AND expires_at > ?", consumed.Id, now).
			Update("consumed_at", now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrAuthFlowConsumed
		}
		consumed.ConsumedAt = &now
		if action != nil {
			if err := action(tx, &consumed); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &consumed, nil
}

func GetUserForQuickLoginTokenIssue(tx *gorm.DB, userId int) (*User, error) {
	var user User
	err := lockForUpdate(tx).Select("id", "status", "group").First(&user, userId).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func DeleteExpiredAuthFlows(now time.Time) error {
	cutoff := now.Add(-AuthFlowDefaultCleanupRetention)
	return DB.Where("expires_at < ? OR (consumed_at IS NOT NULL AND consumed_at < ?)", cutoff, cutoff).
		Delete(&AuthFlow{}).Error
}
