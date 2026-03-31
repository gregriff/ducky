// Package openai adds additional fields and implements behavior of OpenAI LLMs.
package openai

import (
	"fmt"
	"strings"

	"github.com/gregriff/ducky/internal/models"
	"github.com/openai/openai-go/v3/shared"
)

// properties specifies fields unique to OpenAI models.
type properties struct {
	models.Pricing

	// official id from openai's API
	id string
	supportsTemperature,
	supportsReasoning *bool
}

// reasoningEffortMap maps uints to strings used for the reasoningEffort parameter.
// Allows easier migration to new effort levels.
var reasoningEffortMap = map[int]shared.ReasoningEffort{
	1: shared.ReasoningEffortMinimal,
	2: shared.ReasoningEffortLow,
	3: shared.ReasoningEffortMedium,
	4: shared.ReasoningEffortHigh,
}

// these must correspond to the map above.
const (
	minReasoningEffortInt int = 1
	maxReasoningEffortInt int = 4
)

// modelProperties returns a map of OpenAI model names to properties about those models.
var modelProperties = map[string]properties{
	"o3": {
		id: "o3",
		Pricing: models.Pricing{
			PromptCost:   10. / 1_000_000,
			ResponseCost: 40. / 1_000_000,
		},
		supportsReasoning:   new(true),
		supportsTemperature: new(false),
	},
	"o4-mini": {
		id: "o4-mini",
		Pricing: models.Pricing{
			PromptCost:   1.1 / 1_000_000,
			ResponseCost: 4.4 / 1_000_000,
		},
		supportsReasoning:   new(true),
		supportsTemperature: new(false),
	},
	"gpt-4o-mini": {
		id: "gpt-4o-mini",
		Pricing: models.Pricing{
			PromptCost:   .15 / 1_000_000,
			ResponseCost: .075 / 1_000_000,
		},
	},
	"gpt-4o": {
		id: "gpt-4o",
		Pricing: models.Pricing{
			PromptCost:   2.5 / 1_000_000,
			ResponseCost: 10. / 1_000_000,
		},
	},
	"gpt-5": {
		id: "gpt-5",
		Pricing: models.Pricing{
			PromptCost:   1.25 / 1_000_000,
			ResponseCost: 10. / 1_000_000,
		},
		supportsReasoning: new(true),
	},
	"gpt-5-mini": {
		id: "gpt-5-mini",
		Pricing: models.Pricing{
			PromptCost:   .25 / 1_000_000,
			ResponseCost: 2. / 1_000_000,
		},
		supportsReasoning: new(true),
	},
	"gpt-5-nano": {
		id: "gpt-5-nano",
		Pricing: models.Pricing{
			PromptCost:   .05 / 1_000_000,
			ResponseCost: .4 / 1_000_000,
		},
		supportsReasoning: new(true),
	},
}

// ValidateModelName validates that a modelName is one of our supported models. If so, it returns the modelId.
func ValidateModelName(modelName string) error {
	if _, exists := modelProperties[modelName]; !exists {
		var validNames []string
		for name := range modelProperties {
			validNames = append(validNames, name)
		}
		return fmt.Errorf("valid OpenAI models: %s", strings.Join(validNames, ", "))
	}
	return nil
}
