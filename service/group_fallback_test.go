package service

import (
	"errors"
	"reflect"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func TestBuildGroupFallbackChain_EmptyConfig(t *testing.T) {
	cfg := map[string][]string{}
	chain := buildGroupFallbackChain(cfg, "cheap-a")
	if len(chain) != 1 || chain[0] != "cheap-a" {
		t.Fatalf("expected single-element chain for empty config, got %v", chain)
	}
}

func TestBuildGroupFallbackChain_OrderAndDedupe(t *testing.T) {
	cfg := map[string][]string{
		"cheap-a": {"mid-b", "high-c"},
		"mid-b":   {"high-c", "cheap-a"},
	}
	chain := buildGroupFallbackChain(cfg, "cheap-a")
	expected := []string{"cheap-a", "mid-b", "high-c"}
	if len(chain) != len(expected) {
		t.Fatalf("chain length mismatch: got %v, want %v", chain, expected)
	}
	for i := range expected {
		if chain[i] != expected[i] {
			t.Fatalf("chain[%d]=%s, want %s", i, chain[i], expected[i])
		}
	}
}

func TestBuildGroupFallbackChain_StopsWhenNoMoreGroups(t *testing.T) {
	cfg := map[string][]string{
		"cheap-a": {"mid-b"},
	}
	chain := buildGroupFallbackChain(cfg, "mid-b")
	if len(chain) != 1 || chain[0] != "mid-b" {
		t.Fatalf("expected single-element chain for mid-b when fallback missing, got %v", chain)
	}
}

func TestShouldFallbackStatus(t *testing.T) {
	cases := []struct {
		code int
		want bool
	}{
		{500, true},
		{502, true},
		{429, true},
		{408, true},
		{400, false},
		{404, false},
		{401, false},
	}
	for _, c := range cases {
		if got := shouldFallbackStatus(c.code); got != c.want {
			t.Fatalf("status %d fallback expectation mismatch: got %v want %v", c.code, got, c.want)
		}
	}
}

func TestShouldFallbackForError_DefaultAllow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	common.SetContextKey(c, constant.ContextKeyRequestedGroup, "cheap-a")
	if err := setting.UpdateGroupFallbacksByJSONString(`{"cheap-a":["mid-b"]}`); err != nil {
		t.Fatalf("setup group fallbacks: %v", err)
	}
	defer func() {
		_ = setting.UpdateGroupFallbacksByJSONString(`{}`)
	}()
	common.SetContextKey(c, constant.ContextKeyFallbackGroupChain, []string{"cheap-a", "mid-b"})

	err := types.NewOpenAIError(errors.New("upstream overloaded"), types.ErrorCodeBadResponseStatusCode, 503)
	if !ShouldFallbackForError(c, err) {
		t.Fatalf("expected fallback for 503 upstream error")
	}
}

func TestShouldFallbackForError_WhitelistBlock(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	common.SetContextKey(c, constant.ContextKeyRequestedGroup, "cheap-a")
	if err := setting.UpdateGroupFallbacksByJSONString(`{"cheap-a":["mid-b"]}`); err != nil {
		t.Fatalf("setup group fallbacks: %v", err)
	}
	defer func() {
		_ = setting.UpdateGroupFallbacksByJSONString(`{}`)
	}()
	common.SetContextKey(c, constant.ContextKeyFallbackGroupChain, []string{"cheap-a", "mid-b"})

	err := types.NewError(errors.New("bad model id"), types.ErrorCodeModelNotFound)
	if ShouldFallbackForError(c, err) {
		t.Fatalf("expected model-not-found to stay blocked from fallback")
	}
}

func TestAdvanceFallbackGroupOnError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	common.SetContextKey(c, constant.ContextKeyRequestedGroup, "cheap-a")
	if err := setting.UpdateGroupFallbacksByJSONString(`{"cheap-a":["mid-b"]}`); err != nil {
		t.Fatalf("setup group fallbacks: %v", err)
	}
	defer func() {
		_ = setting.UpdateGroupFallbacksByJSONString(`{}`)
	}()
	common.SetContextKey(c, constant.ContextKeyFallbackGroupChain, []string{"cheap-a", "mid-b"})

	retryParam := &RetryParam{
		Ctx:        c,
		TokenGroup: "cheap-a",
		ModelName:  "gpt-4o",
		Retry:      common.GetPointer(1),
	}
	err := types.NewOpenAIError(errors.New("upstream overloaded"), types.ErrorCodeBadResponseStatusCode, 503)
	if !AdvanceFallbackGroupOnError(c, retryParam, err) {
		t.Fatalf("expected fallback advance")
	}
	if got := common.GetContextKeyInt(c, constant.ContextKeyFallbackGroupIndex); got != 1 {
		t.Fatalf("fallback index mismatch: got %d want 1", got)
	}
	if retryParam.GetRetry() != 0 {
		t.Fatalf("retry should reset after fallback advance, got %d", retryParam.GetRetry())
	}
}

func TestBuildModelGroupFallbackCandidates_FiltersExplicitChain(t *testing.T) {
	prev := setting.EnableModelGroupAutoFallback
	setting.SetModelGroupAutoFallback(false)
	defer setting.SetModelGroupAutoFallback(prev)

	chain := []string{"cheap-a", "mid-b", "vip"}
	allowed := map[string]string{
		"cheap-a": "",
		"mid-b":   "",
		"vip":     "",
	}
	modelGroups := []string{"cheap-a", "mid-b"}

	got := buildModelGroupFallbackCandidates("cheap-a", chain, allowed, modelGroups)
	want := []string{"cheap-a", "mid-b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered fallback chain mismatch: got %v want %v", got, want)
	}
}

func TestBuildModelGroupFallbackCandidates_AppendsDynamicFallbacks(t *testing.T) {
	prev := setting.EnableModelGroupAutoFallback
	setting.SetModelGroupAutoFallback(true)
	defer setting.SetModelGroupAutoFallback(prev)

	chain := []string{"cheap-a"}
	allowed := map[string]string{
		"cheap-a": "",
		"stable":  "",
	}
	modelGroups := []string{"cheap-a", "stable"}

	got := buildModelGroupFallbackCandidates("cheap-a", chain, allowed, modelGroups)
	want := []string{"cheap-a", "stable"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dynamic fallback mismatch: got %v want %v", got, want)
	}
}

func TestBuildModelGroupFallbackCandidates_SkipsWhenModelMissing(t *testing.T) {
	prev := setting.EnableModelGroupAutoFallback
	setting.SetModelGroupAutoFallback(true)
	defer setting.SetModelGroupAutoFallback(prev)

	chain := []string{"cheap-a", "stable"}
	allowed := map[string]string{
		"cheap-a": "",
		"stable":  "",
	}

	got := buildModelGroupFallbackCandidates("cheap-a", chain, allowed, nil)
	want := []string{"cheap-a"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected no fallback when model missing: got %v want %v", got, want)
	}
}

func TestBuildModelGroupFallbackCandidates_RespectsUserGroups(t *testing.T) {
	prev := setting.EnableModelGroupAutoFallback
	setting.SetModelGroupAutoFallback(true)
	defer setting.SetModelGroupAutoFallback(prev)

	chain := []string{"cheap-a"}
	allowed := map[string]string{
		"cheap-a": "",
	}
	modelGroups := []string{"cheap-a", "vip"}

	got := buildModelGroupFallbackCandidates("cheap-a", chain, allowed, modelGroups)
	want := []string{"cheap-a"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("felt fallback violated user limits: got %v want %v", got, want)
	}
}
