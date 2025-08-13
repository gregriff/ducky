package openai

import (
	"fmt"
	"strings"

	"github.com/gregriff/ducky/models"
)

type OpenAIModelConfig struct {
	models.Pricing

	// official ID from openai's API
	Id                  string
	SupportsTemperature *bool
	Reasoning           *bool
}

// GetOpenAIModelConfigs returns a map of OpenAI model names to properties about those models
var OpenAIModelConfigs = map[string]OpenAIModelConfig{
	"o3": {
		Id: "o3",
		Pricing: models.Pricing{
			PromptCost:   10. / 1_000_000,
			ResponseCost: 40. / 1_000_000,
		},
		Reasoning:           models.BoolPtr(true),
		SupportsTemperature: models.BoolPtr(false),
	},
	"o4-mini": {
		Id: "o4-mini",
		Pricing: models.Pricing{
			PromptCost:   1.1 / 1_000_000,
			ResponseCost: 4.4 / 1_000_000,
		},
		Reasoning:           models.BoolPtr(true),
		SupportsTemperature: models.BoolPtr(false),
	},
	"gpt-4o-mini": {
		Id: "gpt-4o-mini",
		Pricing: models.Pricing{
			PromptCost:   .15 / 1_000_000,
			ResponseCost: .075 / 1_000_000,
		},
	},
	"gpt-4o": {
		Id: "gpt-4o",
		Pricing: models.Pricing{
			PromptCost:   2.5 / 1_000_000,
			ResponseCost: 10. / 1_000_000,
		},
	},
	"gpt-5": {
		Id: "gpt-5",
		Pricing: models.Pricing{
			PromptCost:   1.25 / 1_000_000,
			ResponseCost: 10. / 1_000_000,
		},
	},
	"gpt-5-mini": {
		Id: "gpt-5-mini",
		Pricing: models.Pricing{
			PromptCost:   .25 / 1_000_000,
			ResponseCost: 2. / 1_000_000,
		},
	},
	"gpt-5-nano": {
		Id: "gpt-5-nano",
		Pricing: models.Pricing{
			PromptCost:   .05 / 1_000_000,
			ResponseCost: .4 / 1_000_000,
		},
	},
}

// ValidateModelName validates that a modelName is one of our supported models. If so, it returns the modelId
func ValidateModelName(modelName string) error {
	if _, exists := OpenAIModelConfigs[modelName]; !exists {
		var validNames []string
		for name := range OpenAIModelConfigs {
			validNames = append(validNames, name)
		}
		err := fmt.Errorf("invalid model name '%s'. Valid options: %s", modelName, strings.Join(validNames, ", "))
		return err
	}
	return nil
}

// GetValidModelNames returns the keys of OpenAIModelConfigs, our supported OpenAI models
func GetValidModelNames() []string {
	var names []string
	for name := range OpenAIModelConfigs {
		names = append(names, name)
	}
	return names
}
