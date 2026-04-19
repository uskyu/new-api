package taskcommon

import "testing"

func TestSanitizeBillingRatiosForOpenAIVideoPathDropsSeconds(t *testing.T) {
	ratios := SanitizeBillingRatiosForRequestPath("/v1/videos", map[string]float64{
		"seconds":    8,
		"resolution": 1.5,
	})
	if _, ok := ratios["seconds"]; ok {
		t.Fatalf("expected seconds ratio to be removed, got %#v", ratios)
	}
	if ratios["resolution"] != 1.5 {
		t.Fatalf("expected resolution ratio to be preserved, got %#v", ratios["resolution"])
	}
}

func TestSanitizeBillingRatiosForOpenAIVideoRemixDropsSeconds(t *testing.T) {
	ratios := SanitizeBillingRatiosForRequestPath("/v1/videos/task_123/remix", map[string]float64{
		"seconds": 8,
		"size":    1.666667,
	})
	if _, ok := ratios["seconds"]; ok {
		t.Fatalf("expected seconds ratio to be removed for remix, got %#v", ratios)
	}
	if ratios["size"] != 1.666667 {
		t.Fatalf("expected size ratio to be preserved, got %#v", ratios["size"])
	}
}

func TestSanitizeBillingRatiosForNonOpenAIVideoPathKeepsSeconds(t *testing.T) {
	ratios := SanitizeBillingRatiosForRequestPath("/kling/v1/videos/text2video", map[string]float64{
		"seconds": 8,
		"size":    1.666667,
	})
	if ratios["seconds"] != 8 {
		t.Fatalf("expected seconds ratio to be kept, got %#v", ratios["seconds"])
	}
	if ratios["size"] != 1.666667 {
		t.Fatalf("expected size ratio to be kept, got %#v", ratios["size"])
	}
}
