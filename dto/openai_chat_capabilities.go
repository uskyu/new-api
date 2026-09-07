package dto

import (
	"strings"
	"time"
)

type OpenAIChatCapabilities struct {
	UseMaxCompletionTokens bool
	UseDeveloperRole       bool
	SupportsTemperature    bool
	SupportsTopP           bool
	SupportsLogProbs       bool
}

// Unknown models do not inherit restrictions from future GPT generations.
func GetOpenAIChatCapabilities(model, effort string) OpenAIChatCapabilities {
	c := OpenAIChatCapabilities{SupportsTemperature: true, SupportsTopP: true, SupportsLogProbs: true}
	if IsOpenAIReasoningOModel(model) {
		c.UseMaxCompletionTokens = true
		c.UseDeveloperRole = !strings.HasPrefix(model, "o1-mini") && !strings.HasPrefix(model, "o1-preview")
		c.SupportsTemperature = false
		return c
	}
	gpt5 := model == "gpt-5" || strings.HasPrefix(model, "gpt-5-") || strings.HasPrefix(model, "gpt-5.")
	if !gpt5 && !isOpenAIChatSnapshot(model, "gpt-6-astra") {
		return c
	}
	c.UseMaxCompletionTokens = true
	c.UseDeveloperRole = true
	sampling := false
	if gpt5 && (effort == "" || effort == "none") {
		for _, base := range []string{"gpt-5.1", "gpt-5.2", "gpt-5.4"} {
			if isOpenAIChatSnapshot(model, base) {
				sampling = true
				break
			}
		}
	}
	c.SupportsTemperature = sampling
	c.SupportsTopP = sampling
	c.SupportsLogProbs = sampling
	return c
}

func isOpenAIChatSnapshot(model, base string) bool {
	if model == base {
		return true
	}
	suffix, ok := strings.CutPrefix(model, base+"-")
	if !ok {
		return false
	}
	_, err := time.Parse(time.DateOnly, suffix)
	return err == nil
}
