package anthropic

import (
	"context"
	"errors"
	"io"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/gregriff/ducky/internal/models"
)

// model encapsulates an Anthropic model and satisfies the models.LLM interface.
type model struct {
	models.BaseLLM
	client             anthropic.Client
	props              properties
	systemPromptObject []anthropic.TextBlockParam
	// TODO: add total session usage field

	// price in dollars. getter should fmt it to cents if small enough. should
	// be updated after each response stream completes, using current model's pricing.
	// should be reset on clear
	totalCost float64

	// whether to use aws bedrock.
	bedrock bool

	// stores each full response.
	fullResponseText *strings.Builder
}

// NewModel creates a new Anthropic Model to be used for response streaming.
func NewModel(
	systemPrompt string,
	maxTokens int,
	modelName string,
	pastMessages *[]models.Message,
	bedrockConfig *models.BedrockConfig,
) (*model, error) {
	// allow message history to persist when user changes model being used
	var messages []models.Message
	if pastMessages != nil {
		messages = *pastMessages
	} else {
		messages = []models.Message{}
	}

	var opts []option.RequestOption
	var err error
	if bedrockConfig != nil {
		opts, err = buildBedrockConfig(context.TODO(), bedrockConfig, opts)
		if err != nil {
			return nil, err
		}
	}

	client := anthropic.NewClient(opts...)
	sb := strings.Builder{}
	sb.Grow(maxTokens)
	m := &model{
		BaseLLM: models.BaseLLM{
			SystemPrompt: systemPrompt,
			MaxTokens:    maxTokens,
			Messages:     messages,
			PromptCount:  0, // TODO: ensure total usage cost is persisted between model changes
		},
		client:             client, // by default uses os.LookupEnv("ANTHROPIC_API_KEY") TODO: use viper config var
		props:              modelProperties[modelName],
		systemPromptObject: []anthropic.TextBlockParam{{Text: systemPrompt}},
		bedrock:            bedrockConfig != nil,
		fullResponseText:   &sb,
	}
	return m, nil
}

func (llm *model) StreamPromptCompletion(ctx context.Context, content string, enableThinking bool, _ *uint8, responseChan chan models.StreamChunk) error {
	defer llm.fullResponseText.Reset()
	defer close(responseChan)

	maxTokens := int64(llm.MaxTokens)
	output := anthropic.OutputConfigParam{}

	// TODO: control this...
	output.Effort = anthropic.OutputConfigEffortMedium
	if maxTokens <= 1024 { // https://docs.anthropic.com/en/docs/build-with-claude/extended-thinking#max-tokens-and-context-window-size
		maxTokens = 3072
	} else {
		maxTokens *= 2
	}

	// disable thinking if using haiku, or the user has said so.
	// cannot disable thinking on opus currently.
	thinking := anthropic.ThinkingConfigParamUnion{}
	if strings.Contains(llm.ModelId(), "haiku") {
		thinking.OfDisabled = &anthropic.ThinkingConfigDisabledParam{}
	} else if !enableThinking && !strings.Contains(llm.ModelId(), "opus") {
		thinking.OfBetweenTools = &anthropic.ThinkingConfigBetweenToolsParam{}
	} else {
		thinking.OfAdaptive = &anthropic.ThinkingConfigAdaptiveParam{
			Display: anthropic.ThinkingConfigAdaptiveDisplayOmitted,
		}
	}

	stream := llm.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:        anthropic.Model(llm.ModelId()),
		System:       llm.systemPromptObject,
		MaxTokens:    maxTokens,
		Messages:     llm.buildMessages(content),
		OutputConfig: output,
		Thinking:     thinking,
	})

	message := anthropic.Message{}
	message.Content = make([]anthropic.ContentBlockUnion, maxTokens/4) // preallocate cuz why not
	inputTokens, outputTokens := 0., 0.
	for stream.Next() {
		event := stream.Current()
		if err := message.Accumulate(event); err != nil {
			return errors.New(stream.Err().Error())
		}

		switch eventVariant := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			switch deltaVariant := eventVariant.Delta.AsAny().(type) {
			case anthropic.TextDelta:
				llm.fullResponseText.WriteString(deltaVariant.Text)
				responseChan <- models.StreamChunk{Reasoning: false, Content: deltaVariant.Text}
			case anthropic.CitationsDelta:
				llm.fullResponseText.WriteString(deltaVariant.Citation.CitedText)
				responseChan <- models.StreamChunk{Reasoning: false, Content: deltaVariant.Citation.CitedText}
			}
		case anthropic.MessageDeltaEvent:
			inputTokens += float64(eventVariant.Usage.InputTokens)
			outputTokens += float64(eventVariant.Usage.OutputTokens)
		}
	}

	if err := stream.Err(); err != nil && err != io.EOF {
		return err
	}

	// update state
	inputCost := llm.props.PromptCost * inputTokens
	outputCost := llm.props.ResponseCost * outputTokens
	llm.totalCost += inputCost + outputCost
	llm.PromptCount++

	if llm.fullResponseText.Len() > 0 {
		llm.Messages = append(llm.Messages, models.Message{Role: "assistant", Content: llm.fullResponseText.String()})
	}
	return nil
}

// buildMessages takes the provider-agnostic []models.Message of the chat history and returns the Anthropic chat history data format.
func (llm *model) buildMessages(newContent string) []anthropic.MessageParam {
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

// given a cost in dollars, return a formatted string to be printed to screen.
func (llm *model) CurrentChatCost() float64 {
	return llm.totalCost
}

func (llm *model) ClearChatHistory() {
	llm.totalCost = 0
	llm.PromptCount = 0
	llm.Messages = []models.Message{}
}

func (llm *model) ChatHistory() []models.Message {
	return llm.Messages
}

func (llm *model) ModelId() string {
	if llm.bedrock {
		return "us.anthropic." + llm.props.id
	}
	return llm.props.id
}

func (llm *model) ModelInfoText() string {
	if llm.bedrock {
		return "[bedrock] " + llm.props.name
		// return lipgloss.NewStyle().
		// Foreground(lipgloss.Color("21")).Render("[bedrock]") + " " + llm.ModelId()
	}
	return llm.props.name
}

func (llm *model) SupportsReasoning() bool {
	return true
}
