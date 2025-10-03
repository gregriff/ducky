// Package models contains interfaces and implemetations of language models from multiple providers
package models

import "context"

// LLM defines fields and behavior of all supported LLMs.
type LLM interface {
	DoStreamPromptCompletion(
		ctx context.Context,
		prompt string,
		enableReasoning bool, // whether the user wants the model to think/reason if supported
		reasoningEffort *uint8, // only to be used for gpt-5 models
		responseChan chan StreamChunk,
	) error
	DoGetCostOfCurrentChat() float64
	DoClearChatHistory()
	DoGetChatHistory() []Message
	DoGetModelId() string
	DoesSupportReasoning() bool
}

func StreamPromptCompletion(ctx context.Context, llm LLM, prompt string, enableReasoning bool, reasoningEffort *uint8, responseChan chan StreamChunk) error {
	if err := llm.DoStreamPromptCompletion(ctx, prompt, enableReasoning, reasoningEffort, responseChan); err != nil {
		return StreamError{ErrMsg: err.Error()}
	}
	return nil
}

func GetCostOfCurrentChat(llm LLM) float64 {
	return llm.DoGetCostOfCurrentChat()
}

func ClearChatHistory(llm LLM) {
	llm.DoClearChatHistory()
}

func GetChatHistory(llm LLM) {
	llm.DoGetChatHistory()
}

func GetModelId(llm LLM) string {
	return llm.DoGetModelId()
}

func SupportsReasoning(llm LLM) bool {
	return llm.DoesSupportReasoning()
}

// BaseLLM defines fields shared by all supported LLMs.
type BaseLLM struct {
	SystemPrompt string
	MaxTokens    int

	Messages    []Message
	PromptCount int
}

// Message encapsulates a chunk of text data returned from the provider's API when streaming a response.
type Message struct {
	Role    string
	Content string
}

// Pricing defines costs per input or output token. They should be defined as `(cost per million) / 1,000,000`.
type Pricing struct {
	PromptCost   float64 // per token
	ResponseCost float64 // per token
}

// BoolPtr is a helper to set optional boolean fields.
func BoolPtr(b bool) *bool {
	return &b
}

// Uint8Ptr is a helper to set optional Uint8 fields.
func Uint8Ptr(i uint8) *uint8 {
	return &i
}

// Bubbletea messsages

// StreamChunk is the type of the channel used for communication between the API response handler and bubbletea (its also a bubbletea Msg).
type StreamChunk struct {
	Reasoning bool
	Content   string
}

// StreamError stores any error that occurred during response streaming.
type StreamError struct{ ErrMsg string }

// Error return the error message of a StreamError.
func (e StreamError) Error() string {
	return e.ErrMsg
}
