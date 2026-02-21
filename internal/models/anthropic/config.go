// Package anthropic adds additional fields and implements behavior of Anthropic LLMs.
package anthropic

import (
	"fmt"
	"strings"

	"github.com/gregriff/ducky/internal/models"
)

// ModelConfig specifies fields unique to Anthropic models.
type ModelConfig struct {
	models.Pricing

	// official ID from anthropic's API
	ID       string
	Thinking *bool
}

// AnthropicModelConfigurations is a map of Anthropic model names to properties about those models. Not to be modified.
var AnthropicModelConfigurations = map[string]ModelConfig{
	"sonnet": {
		ID: "claude-sonnet-4-6",
		Pricing: models.Pricing{
			PromptCost:   3. / 1_000_000,
			ResponseCost: 15. / 1_000_000,
		},
		Thinking: models.BoolPtr(true),
	},
	"haiku": {
		ID: "claude-haiku-4-5",
		Pricing: models.Pricing{
			PromptCost:   1. / 1_000_000,
			ResponseCost: 5. / 1_000_000,
		},
	},
	"opus": {
		ID: "claude-opus-4-6",
		Pricing: models.Pricing{
			PromptCost:   5. / 1_000_000,
			ResponseCost: 25. / 1_000_000,
		},
		Thinking: models.BoolPtr(true),
	},
}

// ValidateModelName validates that a modelName is one of our supported models.
func ValidateModelName(modelName string) error {
	if _, exists := AnthropicModelConfigurations[modelName]; !exists {
		var validNames []string
		for name := range AnthropicModelConfigurations {
			validNames = append(validNames, name)
		}
		return fmt.Errorf("valid Anthropic models: %s", strings.Join(validNames, ", "))
	}
	return nil
}
