package models

// BaseModelConfig defines properties that all supported LLMs have
type BaseModelConfig struct {
	Id           string  // defined by LLM provider API Spec
	PromptCost   float64 // per million tokens
	ResponseCost float64 // per million tokens
}

// BoolPtr is a helper to set optional boolean fields.
func BoolPtr(b bool) *bool {
	return &b
}
