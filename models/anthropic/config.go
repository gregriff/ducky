package anthropic

import (
	"fmt"
	"strings"

	"github.com/gregriff/gpt-cli-go/models"
)

type AnthropicModelConfig struct {
	models.BaseModelConfig
	Thinking *bool
}

// A map of Anthropic model names to properties about those models. Not to be modified
var AnthropicModelConfigurations = map[string]AnthropicModelConfig{
	"sonnet": {
		BaseModelConfig: models.BaseModelConfig{
			Id:           "claude-sonnet-4-20250514",
			PromptCost:   3. / 1_000_000,
			ResponseCost: 15. / 1_000_000,
		},
		Thinking: models.BoolPtr(true),
	},
	"haiku": {
		BaseModelConfig: models.BaseModelConfig{
			Id:           "claude-3-5-haiku-latest",
			PromptCost:   .8 / 1_000_000,
			ResponseCost: 4. / 1_000_000,
		},
	},
	"opus": {
		BaseModelConfig: models.BaseModelConfig{
			Id:           "claude-opus-4-20250514",
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
