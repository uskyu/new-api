package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type registerAPIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func setupUserRegisterControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.User{}, &model.Token{}); err != nil {
		t.Fatalf("failed to migrate register test tables: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func newRegisterContext(t *testing.T, body any) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	payload, err := common.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, recorder
}

func decodeRegisterResponse(t *testing.T, recorder *httptest.ResponseRecorder) registerAPIResponse {
	t.Helper()

	var response registerAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode register response: %v", err)
	}
	return response
}

func TestRegisterCreatesInitialTokenForNewUser(t *testing.T) {
	db := setupUserRegisterControllerTestDB(t)

	prevGenerateDefaultToken := constant.GenerateDefaultToken
	prevRegisterEnabled := common.RegisterEnabled
	prevPasswordRegisterEnabled := common.PasswordRegisterEnabled
	prevEmailVerificationEnabled := common.EmailVerificationEnabled
	t.Cleanup(func() {
		constant.GenerateDefaultToken = prevGenerateDefaultToken
		common.RegisterEnabled = prevRegisterEnabled
		common.PasswordRegisterEnabled = prevPasswordRegisterEnabled
		common.EmailVerificationEnabled = prevEmailVerificationEnabled
	})

	constant.GenerateDefaultToken = true
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false

	ctx, recorder := newRegisterContext(t, map[string]any{
		"username": "newuser01",
		"password": "password1",
	})
	Register(ctx)

	response := decodeRegisterResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var user model.User
	if err := db.Where("username = ?", "newuser01").First(&user).Error; err != nil {
		t.Fatalf("failed to query registered user: %v", err)
	}

	var token model.Token
	if err := db.Where("user_id = ?", user.Id).First(&token).Error; err != nil {
		t.Fatalf("failed to query initial token: %v", err)
	}

	if token.UserId != user.Id {
		t.Fatalf("expected token user_id %d, got %d", user.Id, token.UserId)
	}
	if token.Key == "" {
		t.Fatalf("expected generated token key to be non-empty")
	}
	if token.Group != "" {
		t.Fatalf("expected initial token group to be empty, got %q", token.Group)
	}
}

func TestRegisterDoesNotModifyExistingUserTokens(t *testing.T) {
	db := setupUserRegisterControllerTestDB(t)

	prevGenerateDefaultToken := constant.GenerateDefaultToken
	prevRegisterEnabled := common.RegisterEnabled
	prevPasswordRegisterEnabled := common.PasswordRegisterEnabled
	prevEmailVerificationEnabled := common.EmailVerificationEnabled
	t.Cleanup(func() {
		constant.GenerateDefaultToken = prevGenerateDefaultToken
		common.RegisterEnabled = prevRegisterEnabled
		common.PasswordRegisterEnabled = prevPasswordRegisterEnabled
		common.EmailVerificationEnabled = prevEmailVerificationEnabled
	})

	constant.GenerateDefaultToken = true
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false

	existingUser := &model.User{
		Username:    "existing01",
		Password:    "hashed-password",
		DisplayName: "existing01",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AffCode:     "AFF0",
	}
	if err := db.Create(existingUser).Error; err != nil {
		t.Fatalf("failed to seed existing user: %v", err)
	}

	existingToken := &model.Token{
		UserId:             existingUser.Id,
		Name:               "existing-token",
		Key:                "abcd1234efgh5678ijkl9012mnop3456qrst7890uvwx5678",
		Status:             common.TokenStatusEnabled,
		CreatedTime:        1,
		AccessedTime:       1,
		ExpiredTime:        -1,
		RemainQuota:        500000,
		UnlimitedQuota:     true,
		ModelLimitsEnabled: false,
		Group:              "default",
	}
	if err := db.Create(existingToken).Error; err != nil {
		t.Fatalf("failed to seed existing token: %v", err)
	}

	ctx, recorder := newRegisterContext(t, map[string]any{
		"username": "newuser02",
		"password": "password2",
	})
	Register(ctx)

	response := decodeRegisterResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var existingCount int64
	if err := db.Model(&model.Token{}).Where("user_id = ?", existingUser.Id).Count(&existingCount).Error; err != nil {
		t.Fatalf("failed to count existing user tokens: %v", err)
	}
	if existingCount != 1 {
		t.Fatalf("expected existing user token count to remain 1, got %d", existingCount)
	}

	var totalUsers int64
	if err := db.Model(&model.User{}).Count(&totalUsers).Error; err != nil {
		t.Fatalf("failed to count users: %v", err)
	}
	if totalUsers != 2 {
		t.Fatalf("expected two users after registration, got %d", totalUsers)
	}
}
