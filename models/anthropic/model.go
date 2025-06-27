/*
 * Adds additional fields and implements behavior of Anthropic LLMs
 */
package anthropic

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go" // imported as anthropic
	"github.com/gregriff/gpt-cli-go/models"
)

type AnthropicModel struct {
	models.BaseLLM
	Client      anthropic.Client
	ModelConfig AnthropicModelConfig
	// TODO: add usage field
}

func NewAnthropicModel(systemPrompt string, maxTokens uint32, modelName string, pastMessages *[]models.Message) *AnthropicModel {
	// allow message history to persist when user changes model being used
	var messages []models.Message
	if pastMessages != nil {
		messages = *pastMessages
	} else {
		messages = []models.Message{}
	}

	return &AnthropicModel{
		BaseLLM: models.BaseLLM{
			SystemPrompt: systemPrompt,
			MaxTokens:    maxTokens,
			Messages:     messages,
			PromptCount:  0, // TODO: ensure total usage cost is persisted between model changes
		},
		Client:      anthropic.NewClient(), // by default uses os.LookupEnv("ANTHROPIC_API_KEY") TODO: use viper config var
		ModelConfig: AnthropicModelConfigurations[modelName],
	}
}

func (llm *AnthropicModel) DoStreamPromptCompletion(content string, enableReasoning bool, ch chan string) {
	defer close(ch)

	// create per-prompt properties (user will be able to change these at any time)
	maxTokens := int64(llm.MaxTokens)
	var thinking anthropic.ThinkingConfigParamUnion
	if thinkingEnabled := llm.ModelConfig.Thinking; thinkingEnabled != nil && *thinkingEnabled && enableReasoning {
		thinking = anthropic.ThinkingConfigParamOfEnabled(maxTokens)
		if maxTokens <= 1024 { // https://docs.anthropic.com/en/docs/build-with-claude/extended-thinking#max-tokens-and-context-window-size
			maxTokens = 2048
		} else {
			maxTokens *= 2
		}
	} else {
		disabled := anthropic.NewThinkingConfigDisabledParam()
		thinking = anthropic.ThinkingConfigParamUnion{OfDisabled: &disabled}
	}

	llm.Messages = append(llm.Messages, models.Message{Role: "user", Content: content})
	stream := llm.Client.Messages.NewStreaming(context.TODO(), anthropic.MessageNewParams{
		// Model:     anthropic.ModelClaude3_7SonnetLatest,
		Model:     anthropic.Model(llm.ModelConfig.Id),
		System:    []anthropic.TextBlockParam{{Text: llm.SystemPrompt}},
		MaxTokens: maxTokens,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(content)),
		},
		Thinking: thinking,
	})

	message := anthropic.Message{}
	for stream.Next() {
		event := stream.Current()
		err := message.Accumulate(event)
		if err != nil {
			panic(err)
		}

		switch eventVariant := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			switch deltaVariant := eventVariant.Delta.AsAny().(type) {
			case anthropic.ThinkingDelta:
				ch <- deltaVariant.Thinking
			case anthropic.TextDelta:
				ch <- deltaVariant.Text
			case anthropic.CitationsDelta:
				ch <- deltaVariant.Citation.DocumentTitle
			}
		}
	}

	if stream.Err() != nil {
		panic(stream.Err())
	}

	// update state
	llm.PromptCount += 1
	fullMessageChain := stream.Current().Message.Content

	// TODO: figure out this block in order to preserve history
	fmt.Println(fullMessageChain)
	if len(fullMessageChain) > 0 {
		fullResponse := fullMessageChain[len(fullMessageChain)-1]
		fmt.Println(fullMessageChain[0])
		llm.Messages = append(llm.Messages, models.Message{Role: "assistant", Content: fullResponse.Text})
	}
}

func (llm *AnthropicModel) DoGetCostOfCurrentChat() float64 {
	return -1.
}

func (llm *AnthropicModel) DoClearChatHistory() {
	llm.PromptCount = 0
	llm.Messages = []models.Message{}
	// TODO: reset usage
}
