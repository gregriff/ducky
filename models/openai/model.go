/*
 * Adds additional fields and implements behavior of OpenAI LLMs
 */
package openai

import (
	"context"
	"log"

	"github.com/gregriff/ducky/models"
	openai "github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/packages/param"
	"github.com/openai/openai-go/v2/responses"
	"github.com/openai/openai-go/v2/shared"
)

// OpenAIModel satisfies the models.LLM interface
type OpenAIModel struct {
	models.BaseLLM
	Client       openai.Client
	ModelConfig  OpenAIModelConfig
	SystemPrompt string
	// TODO: add usage field
}

func NewModel(systemPrompt string, maxTokens int, modelName string, pastMessages *[]models.Message) *OpenAIModel {
	// allow message history to persist when user changes model being used
	var messages []models.Message
	if pastMessages != nil {
		messages = *pastMessages
	} else {
		messages = []models.Message{}
	}

	return &OpenAIModel{
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

func (llm *OpenAIModel) DoStreamPromptCompletion(content string, enableReasoning bool, responseChan chan models.StreamChunk) error {
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
		reasoning = openai.ReasoningParam{Effort: shared.ReasoningEffortMinimal} // can be minimal, low, medium, high
		// if maxTokens <= 1024 {
		// 	maxTokens = 2048
		// } else {
		// 	maxTokens *= 2
		// }
	}
	log.Println("init stream")

	// https://pkg.go.dev/github.com/openai/openai-go/v2/responses#ResponseNewParams
	stream := llm.Client.Responses.NewStreaming(context.TODO(), responses.ResponseNewParams{
		Model:           shared.ResponsesModel(llm.ModelConfig.Id),
		Input:           buildMessages(llm.Messages, content),
		Reasoning:       reasoning,
		Instructions:    param.Opt[string]{Value: llm.SystemPrompt},
		MaxOutputTokens: param.Opt[int64]{Value: maxTokens},
		Store:           param.Opt[bool]{Value: false},
		// Include:         []responses.ResponseIncludable{"reasoning.encrypted_content"},
	})

	// message := responses.ResponseOutputMessage{}
	for stream.Next() {
		chunk := stream.Current()
		// responses.ResponseOutputText  // a helper

		// if len(chunk.Delta) > 0 {
		// 	println(chunk.Choices[0].Delta.Content)
		// }

		switch eventVariant := chunk.AsAny().(type) {
		case responses.ResponseCompletedEvent:
			log.Println("response completed")
		case responses.ResponseCreatedEvent:
			log.Println("response created")
		case responses.ResponseErrorEvent:
			log.Println("response error")
		case responses.ResponseFailedEvent:
			log.Println("response failed")
		case responses.ResponseReasoningSummaryTextDoneEvent:
			log.Println("response reasoning summary text done event: ")
		case responses.ResponseReasoningTextDoneEvent:
			log.Println("response reasoning text done event: ")
		case responses.ResponseReasoningTextDeltaEvent:
			log.Println("reasoning delta event: ", eventVariant.Delta)
			fullResponseText += eventVariant.Delta
			responseChan <- models.StreamChunk{Reasoning: true, Content: chunk.Delta}
		case responses.ResponseTextDeltaEvent:
			log.Println("response delta event: ", eventVariant.Delta)
			fullResponseText += eventVariant.Delta
			responseChan <- models.StreamChunk{Reasoning: false, Content: chunk.Delta}
		}
	}

	if stream.Err() != nil {
		return models.StreamError{ErrMsg: stream.Err().Error()}
	}

	// update state
	llm.PromptCount += 1

	if len(fullResponseText) > 0 {
		llm.Messages = append(llm.Messages, models.Message{Role: "assistant", Content: fullResponseText})
	}
	return nil
}

// buildMessages takes the provider-agnostic []models.Message of the chat history and returns the OpenAI chat history data format
func buildMessages(history []models.Message, newContent string) responses.ResponseNewParamsInputUnion {
	messages := make([]responses.ResponseInputItemUnionParam, 0, len(history)+1)
	var (
		currentResponseInputParam  responses.ResponseInputItemUnionParam
		currentMessageContentParam responses.EasyInputMessageContentUnionParam
	)

	// Add conversation history
	for _, msg := range history {
		currentMessageContentParam = responses.EasyInputMessageContentUnionParam{OfString: param.Opt[string]{Value: msg.Content}}

		switch msg.Role {
		case "user":
			currentResponseInputParam = responses.ResponseInputItemUnionParam{OfMessage: &responses.EasyInputMessageParam{Content: currentMessageContentParam, Role: "user"}}
		case "assistant":
			currentResponseInputParam = responses.ResponseInputItemUnionParam{OfMessage: &responses.EasyInputMessageParam{Content: currentMessageContentParam, Role: "assistant"}}
		}
		messages = append(messages, currentResponseInputParam)
	}

	// Add current message
	currentMessageContentParam = responses.EasyInputMessageContentUnionParam{OfString: param.Opt[string]{Value: newContent}}
	messages = append(messages, responses.ResponseInputItemUnionParam{OfMessage: &responses.EasyInputMessageParam{Content: currentMessageContentParam, Role: "user"}})

	return responses.ResponseNewParamsInputUnion{OfInputItemList: messages}
}

func (llm *OpenAIModel) DoGetCostOfCurrentChat() float64 {
	return -1.
}

func (llm *OpenAIModel) DoClearChatHistory() {
	llm.PromptCount = 0
	llm.Messages = []models.Message{}
	// TODO: reset usage
}

func (llm *OpenAIModel) DoGetChatHistory() []models.Message {
	return llm.Messages
}

func (llm *OpenAIModel) DoGetModelId() string {
	return llm.ModelConfig.Id
}

func (llm *OpenAIModel) DoesSupportReasoning() bool {
	thinking := llm.ModelConfig.SupportsReasoning
	if thinking != nil && *thinking {
		return true
	}
	return false
}
