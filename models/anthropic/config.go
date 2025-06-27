package anthropic

import (
	"github.com/gregriff/gpt-cli-go/models"
)

type AnthropicModelConfig struct {
	models.BaseModelConfig
	Thinking *bool
}

// GetAnthropicModelConfigs returns properties about an Anthropic Model given a shorthand version of its ID.
func GetAnthropicModelConfigs() map[string]AnthropicModelConfig {
	return map[string]AnthropicModelConfig{
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
}
