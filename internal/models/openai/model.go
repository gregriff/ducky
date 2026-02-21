package openai

import (
	"context"
	"errors"
	"fmt"

	"github.com/gregriff/ducky/internal/math"
	"github.com/gregriff/ducky/internal/models"
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

// Model encapsulated an OpenAI model and satisfies the models.LLM interface.
type Model struct {
	models.BaseLLM
	Client       openai.Client
	ModelConfig  ModelConfig
	SystemPrompt string
	// TODO: add usage field
}

// NewModel creates a new OpenAI model to be used for response streaming.
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
		Client:       openai.NewClient(), // by default uses os.LookupEnv("OPENAI_API_KEY") TODO: use viper config var
		ModelConfig:  OpenAIModelConfigurations[modelName],
		SystemPrompt: systemPrompt,
	}
}

func (llm *Model) DoStreamPromptCompletion(ctx context.Context, content string, enableReasoning bool, reasoningEffort *uint8, responseChan chan models.StreamChunk) error {
	defer close(responseChan)

	var (
		// per-prompt properties (user will be able to change these at any time)
		maxTokens          int64
		reasoning          openai.ReasoningParam
		reasoningSupported *bool
	)

	maxTokens = int64(llm.MaxTokens)
	fullResponseText := ""

	if reasoningSupported = llm.ModelConfig.SupportsReasoning; reasoningSupported != nil && *reasoningSupported && enableReasoning {
		var (
			effortNormalized int
			effortParam      shared.ReasoningEffort
		)

		if reasoningEffort != nil {
			effortNormalized = math.Clamp(
				int(*reasoningEffort),
				MinReasoningEffortInt,
				MaxReasoningEffortInt,
			)
			effortParam = ReasoningEffortMap[effortNormalized]
		} else {
			// this should never run because viper sets a default effort flag
			effortParam = shared.ReasoningEffortMinimal
		}
		reasoning = openai.ReasoningParam{Effort: effortParam} // can be minimal, low, medium, high
		// if maxTokens <= 1024 {
		// 	maxTokens = 2048
		// } else {
		// 	maxTokens *= 2
		// }
	}

	// TODO: add reasoning summary support

	// https://pkg.go.dev/github.com/openai/openai-go/v2/responses#ResponseNewParams
	stream := llm.Client.Responses.NewStreaming(ctx, responses.ResponseNewParams{
		Model:           llm.ModelConfig.ID,
		Input:           llm.buildMessages(content),
		Reasoning:       reasoning,
		Instructions:    param.Opt[string]{Value: llm.SystemPrompt},
		MaxOutputTokens: param.Opt[int64]{Value: maxTokens},
		Store:           param.Opt[bool]{Value: false},
		// Include:         []responses.ResponseIncludable{"reasoning.encrypted_content"},
	})

	for stream.Next() {
		chunk := stream.Current()
		// responses.ResponseOutputText  // a helper

		switch eventVariant := chunk.AsAny().(type) {
		// case responses.ResponseCompletedEvent:
		// 	log.Println("response completed")
		// case responses.ResponseCreatedEvent:
		// 	log.Println("response created")
		// case responses.ResponseErrorEvent:
		// 	log.Println("response error")
		// case responses.ResponseFailedEvent:
		// 	log.Println("response failed")
		// case responses.ResponseReasoningSummaryTextDoneEvent:
		// 	log.Println("response reasoning summary text done event: ")
		// case responses.ResponseReasoningTextDoneEvent:
		// 	log.Println("response reasoning text done event: ")
		case responses.ResponseReasoningTextDeltaEvent:
			fullResponseText += eventVariant.Delta
			responseChan <- models.StreamChunk{Reasoning: true, Content: chunk.Delta}
		case responses.ResponseTextDeltaEvent:
			fullResponseText += eventVariant.Delta
			responseChan <- models.StreamChunk{Reasoning: false, Content: chunk.Delta}
		}
	}

	if stream.Err() != nil {
		return errors.New(stream.Err().Error())
	}

	// update state
	llm.PromptCount++

	if len(fullResponseText) > 0 {
		llm.Messages = append(llm.Messages, models.Message{Role: "assistant", Content: fullResponseText})
	}
	return nil
}

// buildMessages takes the provider-agnostic []models.Message of the chat history and returns the OpenAI chat history data format.
func (llm *Model) buildMessages(newContent string) responses.ResponseNewParamsInputUnion {
	messages := make([]responses.ResponseInputItemUnionParam, 0, len(llm.Messages)+1)
	var (
		currentResponseInputParam  responses.ResponseInputItemUnionParam
		currentMessageContentParam responses.EasyInputMessageContentUnionParam
		msg                        models.Message
	)

	// Add conversation history
	for i := range len(llm.Messages) {
		msg = llm.Messages[i]
		currentMessageContentParam = responses.EasyInputMessageContentUnionParam{OfString: param.Opt[string]{Value: msg.Content}}

		switch msg.Role {
		case "user":
			currentResponseInputParam = responses.ResponseInputItemUnionParam{OfMessage: &responses.EasyInputMessageParam{Content: currentMessageContentParam, Role: "user"}}
		case "assistant":
			currentResponseInputParam = responses.ResponseInputItemUnionParam{OfMessage: &responses.EasyInputMessageParam{Content: currentMessageContentParam, Role: "assistant"}}
		default:
			panic(fmt.Sprintf("Add support for this type of message:%s", msg.Role))
		}
		messages = append(messages, currentResponseInputParam)
	}

	// Add current message
	currentMessageContentParam = responses.EasyInputMessageContentUnionParam{OfString: param.Opt[string]{Value: newContent}}
	messages = append(messages, responses.ResponseInputItemUnionParam{OfMessage: &responses.EasyInputMessageParam{Content: currentMessageContentParam, Role: "user"}})

	return responses.ResponseNewParamsInputUnion{OfInputItemList: messages}
}

func (llm *Model) DoGetCostOfCurrentChat() float64 {
	return 0
}

func (llm *Model) DoClearChatHistory() {
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
	if reasoning := llm.ModelConfig.SupportsReasoning; reasoning != nil && *reasoning {
		return true
	}
	return false
}
