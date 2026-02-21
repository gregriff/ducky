package anthropic

import (
	"context"
	"errors"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/gregriff/ducky/internal/models"
)

// Model encapsulates an Anthropic model and satisfies the models.LLM interface.
type Model struct {
	models.BaseLLM
	Client             anthropic.Client
	ModelConfig        ModelConfig
	SystemPromptObject []anthropic.TextBlockParam
	// TODO: add usage field

	// price in dollars. getter should fmt it to cents if small enough. should
	// be updated after each response stream completes, using current model's pricing.
	// should be reset on clear
	totalCost float64
}

// NewModel creates a new Anthropic Model to be used for response streaming.
func NewModel(systemPrompt string, maxTokens int, modelName string, pastMessages *[]models.Message) *Model {
	// allow message history to persist when user changes model being used
	var messages []models.Message
	if pastMessages != nil {
		messages = *pastMessages
	} else {
		messages = []models.Message{}
	}

	return &Model{
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

func (llm *Model) DoStreamPromptCompletion(ctx context.Context, content string, enableThinking bool, _ *uint8, responseChan chan models.StreamChunk) error {
	defer close(responseChan)

	var (
		// per-prompt properties (user will be able to change these at any time)
		maxTokens         int64
		thinking          anthropic.ThinkingConfigParamUnion
		thinkingSupported *bool
	)

	maxTokens = int64(llm.MaxTokens)
	fullResponseText := ""
	if thinkingSupported = llm.ModelConfig.Thinking; thinkingSupported != nil && *thinkingSupported && enableThinking {
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

	stream := llm.Client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(llm.ModelConfig.ID),
		System:    llm.SystemPromptObject,
		MaxTokens: maxTokens,
		Messages:  llm.buildMessages(content),
		Thinking:  thinking,
	})

	message := anthropic.Message{}
	message.Content = make([]anthropic.ContentBlockUnion, maxTokens/4) // preallocate cuz why not
	inputTokens, outputTokens := 0., 0.
	for stream.Next() {
		event := stream.Current()
		err := message.Accumulate(event)
		if err != nil {
			return errors.New(stream.Err().Error())
		}

		switch eventVariant := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			switch deltaVariant := eventVariant.Delta.AsAny().(type) {
			case anthropic.ThinkingDelta:
				fullResponseText += deltaVariant.Thinking
				responseChan <- models.StreamChunk{Reasoning: true, Content: deltaVariant.Thinking}
			case anthropic.TextDelta:
				fullResponseText += deltaVariant.Text
				responseChan <- models.StreamChunk{Reasoning: false, Content: deltaVariant.Text}
			case anthropic.CitationsDelta:
				fullResponseText += deltaVariant.Citation.CitedText
				responseChan <- models.StreamChunk{Reasoning: false, Content: deltaVariant.Citation.CitedText}
			}
		case anthropic.MessageDeltaEvent:
			inputTokens += float64(eventVariant.Usage.InputTokens)
			outputTokens += float64(eventVariant.Usage.OutputTokens)
		}
	}

	if stream.Err() != nil {
		return errors.New(stream.Err().Error())
	}

	inputCost := llm.ModelConfig.PromptCost * inputTokens
	outputCost := llm.ModelConfig.ResponseCost * outputTokens
	llm.totalCost += inputCost + outputCost

	// update state
	llm.PromptCount++

	if len(fullResponseText) > 0 {
		llm.Messages = append(llm.Messages, models.Message{Role: "assistant", Content: fullResponseText})
	}
	return nil
}

// buildMessages takes the provider-agnostic []models.Message of the chat history and returns the Anthropic chat history data format.
func (llm *Model) buildMessages(newContent string) []anthropic.MessageParam {
	messages := make([]anthropic.MessageParam, 0, len(llm.Messages)+1)
	var msg models.Message

	// Add conversation history
	for i := range len(llm.Messages) {
		msg = llm.Messages[i]
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

// given a cost in dollars, return a formatted string to be printed to screen
func (llm *Model) DoGetCostOfCurrentChat() float64 {
	return llm.totalCost
}

func (llm *Model) DoClearChatHistory() {
	llm.totalCost = 0
	llm.PromptCount = 0
	llm.Messages = []models.Message{}
	// TODO: reset usage
}

func (llm *Model) DoGetChatHistory() []models.Message {
	return llm.Messages
}

func (llm *Model) DoGetModelId() string {
	return llm.ModelConfig.ID
}

func (llm *Model) DoesSupportReasoning() bool {
	if thinking := llm.ModelConfig.Thinking; thinking != nil && *thinking {
		return true
	}
	return false
}
