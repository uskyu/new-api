package ratio_setting

import "testing"

func TestGPTCompletionRatioCanBeOverridden(t *testing.T) {
	InitRatioSettings()
	t.Cleanup(InitRatioSettings)

	if err := UpdateCompletionRatioByJSONString(`{"gpt-5":3.25,"chatgpt-4o-latest":2.25}`); err != nil {
		t.Fatalf("UpdateCompletionRatioByJSONString returned error: %v", err)
	}

	if ratio := GetCompletionRatio("gpt-5"); ratio != 3.25 {
		t.Fatalf("expected gpt-5 completion ratio from config, got %v", ratio)
	}

	info := GetCompletionRatioInfo("gpt-5")
	if info.Locked {
		t.Fatalf("expected gpt-5 completion ratio to be editable")
	}
	if info.Ratio != 3.25 {
		t.Fatalf("expected gpt-5 completion ratio info from config, got %v", info.Ratio)
	}

	if ratio := GetCompletionRatio("chatgpt-4o-latest"); ratio != 2.25 {
		t.Fatalf("expected chatgpt-4o-latest completion ratio from config, got %v", ratio)
	}
}

func TestGPTCompletionRatioDefaultIsEditableWhenUnset(t *testing.T) {
	InitRatioSettings()
	t.Cleanup(InitRatioSettings)

	if err := UpdateCompletionRatioByJSONString(`{}`); err != nil {
		t.Fatalf("UpdateCompletionRatioByJSONString returned error: %v", err)
	}

	info := GetCompletionRatioInfo("gpt-5")
	if info.Locked {
		t.Fatalf("expected unset gpt-5 completion ratio default to be editable")
	}
	if info.Ratio != 8 {
		t.Fatalf("expected gpt-5 default completion ratio, got %v", info.Ratio)
	}
	if ratio := GetCompletionRatio("gpt-5"); ratio != 8 {
		t.Fatalf("expected gpt-5 default completion ratio, got %v", ratio)
	}
}

func TestNonGPTCompletionRatioRemainsLocked(t *testing.T) {
	InitRatioSettings()
	t.Cleanup(InitRatioSettings)

	if err := UpdateCompletionRatioByJSONString(`{"claude-3-sonnet-20240229":2}`); err != nil {
		t.Fatalf("UpdateCompletionRatioByJSONString returned error: %v", err)
	}

	if ratio := GetCompletionRatio("claude-3-sonnet-20240229"); ratio != 5 {
		t.Fatalf("expected hardcoded claude ratio to remain locked, got %v", ratio)
	}

	info := GetCompletionRatioInfo("claude-3-sonnet-20240229")
	if !info.Locked {
		t.Fatalf("expected claude completion ratio to remain locked")
	}
	if info.Ratio != 5 {
		t.Fatalf("expected hardcoded claude ratio in info, got %v", info.Ratio)
	}
}
