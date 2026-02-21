// Package openai adds additional fields and implements behavior of OpenAI LLMs.
package openai

import (
	"fmt"
	"strings"

	"github.com/gregriff/ducky/internal/models"
	"github.com/openai/openai-go/v3/shared"
)

// ModelConfig specifies fields unique to OpenAI models.
type ModelConfig struct {
	models.Pricing

	// official ID from openai's API
	ID                  string
	SupportsTemperature *bool
	SupportsReasoning   *bool
}

// ReasoningEffortMap maps uints to strings used for the reasoningEffort parameter.
// Allows easier migration to new effort levels.
var ReasoningEffortMap = map[int]shared.ReasoningEffort{
	1: shared.ReasoningEffortMinimal,
	2: shared.ReasoningEffortLow,
	3: shared.ReasoningEffortMedium,
	4: shared.ReasoningEffortHigh,
}

// these must correspond to the map above.
const (
	MinReasoningEffortInt int = 1
	MaxReasoningEffortInt int = 4
)

// OpenAIModelConfigurations returns a map of OpenAI model names to properties about those models.
var OpenAIModelConfigurations = map[string]ModelConfig{
	"o3": {
		ID: "o3",
		Pricing: models.Pricing{
			PromptCost:   10. / 1_000_000,
			ResponseCost: 40. / 1_000_000,
		},
		SupportsReasoning:   models.BoolPtr(true),
		SupportsTemperature: models.BoolPtr(false),
	},
	"o4-mini": {
		ID: "o4-mini",
		Pricing: models.Pricing{
			PromptCost:   1.1 / 1_000_000,
			ResponseCost: 4.4 / 1_000_000,
		},
		SupportsReasoning:   models.BoolPtr(true),
		SupportsTemperature: models.BoolPtr(false),
	},
	"gpt-4o-mini": {
		ID: "gpt-4o-mini",
		Pricing: models.Pricing{
			PromptCost:   .15 / 1_000_000,
			ResponseCost: .075 / 1_000_000,
		},
	},
	"gpt-4o": {
		ID: "gpt-4o",
		Pricing: models.Pricing{
			PromptCost:   2.5 / 1_000_000,
			ResponseCost: 10. / 1_000_000,
		},
	},
	"gpt-5": {
		ID: "gpt-5",
		Pricing: models.Pricing{
			PromptCost:   1.25 / 1_000_000,
			ResponseCost: 10. / 1_000_000,
		},
		SupportsReasoning: models.BoolPtr(true),
	},
	"gpt-5-mini": {
		ID: "gpt-5-mini",
		Pricing: models.Pricing{
			PromptCost:   .25 / 1_000_000,
			ResponseCost: 2. / 1_000_000,
		},
		SupportsReasoning: models.BoolPtr(true),
	},
	"gpt-5-nano": {
		ID: "gpt-5-nano",
		Pricing: models.Pricing{
			PromptCost:   .05 / 1_000_000,
			ResponseCost: .4 / 1_000_000,
		},
		SupportsReasoning: models.BoolPtr(true),
	},
}

// ValidateModelName validates that a modelName is one of our supported models. If so, it returns the modelId.
func ValidateModelName(modelName string) error {
	if _, exists := OpenAIModelConfigurations[modelName]; !exists {
		var validNames []string
		for name := range OpenAIModelConfigurations {
			validNames = append(validNames, name)
		}
		return fmt.Errorf("valid OpenAI models: %s", strings.Join(validNames, ", "))
	}
	return nil
}
