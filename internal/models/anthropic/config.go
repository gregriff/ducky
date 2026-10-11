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
	id string

	// name shown to user
	name string
}

// modelProperties is a map of Anthropic model names to properties about those models. Not to be modified.
var modelProperties = map[string]properties{
	"sonnet": {
		id:   "claude-sonnet-5-5",
		name: "sonnet-5-5",
		Pricing: models.Pricing{
			PromptCost:   2. / 1_000_000,
			ResponseCost: 10. / 1_000_000,
		},
	},
	"haiku": {
		id:   "claude-haiku-5-5",
		name: "haiku-5-5",
		Pricing: models.Pricing{
			PromptCost:   .1 / 1_000_000,
			ResponseCost: .5 / 1_000_000,
		},
	},
	"opus": {
		id:   "claude-opus-5-5",
		name: "opus-5-5",
		Pricing: models.Pricing{
			PromptCost:   4. / 1_000_000,
			ResponseCost: 20. / 1_000_000,
		},
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
