package anthropic

import (
	"fmt"
	"strings"

	"github.com/gregriff/ducky/models"
)

type AnthropicModelConfig struct {
	models.Pricing

	// official ID from anthropic's API
	Id       string
	Thinking *bool
}

// A map of Anthropic model names to properties about those models. Not to be modified
var AnthropicModelConfigurations = map[string]AnthropicModelConfig{
	"sonnet": {
		Id: "claude-sonnet-4-20250514",
		Pricing: models.Pricing{
			PromptCost:   3. / 1_000_000,
			ResponseCost: 15. / 1_000_000,
		},
		Thinking: models.BoolPtr(true),
	},
	"haiku": {
		Id: "claude-3-5-haiku-latest",
		Pricing: models.Pricing{
			PromptCost:   .8 / 1_000_000,
			ResponseCost: 4. / 1_000_000,
		},
	},
	"opus": {
		Id: "claude-opus-4-20250514",
		Pricing: models.Pricing{
			PromptCost:   15. / 1_000_000,
			ResponseCost: 75. / 1_000_000,
		},
	},
}

// ValidateModelName validates that a modelName is one of our supported models
func ValidateModelName(modelName string) error {
	if _, exists := AnthropicModelConfigurations[modelName]; !exists {
		var validNames []string
		for name := range AnthropicModelConfigurations {
			validNames = append(validNames, name)
		}
		return fmt.Errorf("invalid model name '%s'. Valid options: %s", modelName, strings.Join(validNames, ", "))
	}
	return nil
}

// GetValidModelNames returns the keys of AnthropicModelConfigurations, our supported Anthropic models
func GetValidModelNames() []string {
	var names []string
	for name := range AnthropicModelConfigurations {
		names = append(names, name)
	}
	return names
}
