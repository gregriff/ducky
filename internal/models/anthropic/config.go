// Package anthropic adds additional fields and implements behavior of Anthropic LLMs.
package anthropic

import (
	"fmt"
	"strings"

	"github.com/gregriff/ducky/internal/models"
)

// properties specifies fields unique to Anthropic models.
type properties struct {
	models.Pricing

	// official id from anthropic's API
	id,
	name string
	thinking *bool
}

// modelProperties is a map of Anthropic model names to properties about those models. Not to be modified.
var modelProperties = map[string]properties{
	"sonnet": {
		id:   "claude-sonnet-4-6",
		name: "sonnet-4.6",
		Pricing: models.Pricing{
			PromptCost:   3. / 1_000_000,
			ResponseCost: 15. / 1_000_000,
		},
		thinking: new(true),
	},
	"haiku": {
		id:   "claude-haiku-4-5",
		name: "haiku-4.6",
		Pricing: models.Pricing{
			PromptCost:   1. / 1_000_000,
			ResponseCost: 5. / 1_000_000,
		},
	},
	"opus": {
		id:   "claude-opus-4-6",
		name: "opus-4.6",
		Pricing: models.Pricing{
			PromptCost:   5. / 1_000_000,
			ResponseCost: 25. / 1_000_000,
		},
		thinking: new(true),
	},
}

// ValidateModelName validates that a modelName is one of our supported models.
func ValidateModelName(modelName string) error {
	if _, exists := modelProperties[modelName]; !exists {
		var validNames []string
		for name := range modelProperties {
			validNames = append(validNames, name)
		}
		return fmt.Errorf("valid Anthropic models: %s", strings.Join(validNames, ", "))
	}
	return nil
}
