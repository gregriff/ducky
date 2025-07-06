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
	Client             anthropic.Client
	ModelConfig        AnthropicModelConfig
	SystemPromptObject []anthropic.TextBlockParam
	// TODO: add usage field
}

func NewAnthropicModel(systemPrompt string, maxTokens int, modelName string, pastMessages *[]models.Message) *AnthropicModel {
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
		Client:             anthropic.NewClient(), // by default uses os.LookupEnv("ANTHROPIC_API_KEY") TODO: use viper config var
		ModelConfig:        AnthropicModelConfigurations[modelName],
		SystemPromptObject: []anthropic.TextBlockParam{{Text: systemPrompt}},
	}
}

func (llm *AnthropicModel) DoStreamPromptCompletion(content string, enableReasoning bool, ch chan string) {
	defer close(ch)

	var (
		// per-prompt properties (user will be able to change these at any time)
		maxTokens       int64
		thinking        anthropic.ThinkingConfigParamUnion
		thinkingEnabled *bool
	)

	maxTokens = int64(llm.MaxTokens)
	fullResponseText := ""
	if thinkingEnabled = llm.ModelConfig.Thinking; thinkingEnabled != nil && *thinkingEnabled && enableReasoning {
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

	stream := llm.Client.Messages.NewStreaming(context.TODO(), anthropic.MessageNewParams{
		Model:     anthropic.Model(llm.ModelConfig.Id),
		System:    llm.SystemPromptObject,
		MaxTokens: maxTokens,
		Messages:  buildMessages(llm.Messages, content),
		Thinking:  thinking,
	})

	message := anthropic.Message{}
	for stream.Next() {
		event := stream.Current()
		err := message.Accumulate(event)
		if err != nil {
			ch <- fmt.Sprintf("\n\n[Error: %v]", stream.Err())
		}

		switch eventVariant := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			switch deltaVariant := eventVariant.Delta.AsAny().(type) {
			case anthropic.ThinkingDelta:
				ch <- deltaVariant.Thinking
			case anthropic.TextDelta:
				fullResponseText += deltaVariant.Text
				ch <- deltaVariant.Text
			case anthropic.CitationsDelta:
				ch <- deltaVariant.Citation.DocumentTitle
			}
		}
	}

	if stream.Err() != nil {
		ch <- fmt.Sprintf("\n\n[Error: %v]", stream.Err())
	}

	// update state
	llm.PromptCount += 1

	if len(fullResponseText) > 0 {
		llm.Messages = append(llm.Messages, models.Message{Role: "assistant", Content: fullResponseText})
	}
}

// buildMessages takes the vendor-agnostic []models.Message of the chat history and returns the Anthropic chat history data format
func buildMessages(history []models.Message, newContent string) []anthropic.MessageParam {
	messages := make([]anthropic.MessageParam, 0, len(history)+1)

	// Add conversation history
	for _, msg := range history {
		switch msg.Role {
		case "user":
			messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
		case "assistant":
			messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content)))
		}
	}

	// Add current message
	messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(newContent)))

	return messages
}

func (llm *AnthropicModel) DoGetCostOfCurrentChat() float64 {
	return -1.
}

func (llm *AnthropicModel) DoClearChatHistory() {
	llm.PromptCount = 0
	llm.Messages = []models.Message{}
	// TODO: reset usage
}
