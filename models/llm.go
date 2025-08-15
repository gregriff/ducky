/*
 * Defines fields and behavior of all supported LLMs
 */
package models

type LLM interface {
	DoStreamPromptCompletion(
		prompt string,
		enableReasoning bool, // whether the user wants the model to think/reason if supported
		responseChan chan StreamChunk,
	) error
	DoGetCostOfCurrentChat() float64
	DoClearChatHistory()
	DoGetChatHistory() []Message
	DoGetModelId() string
	DoesSupportReasoning() bool
}

func StreamPromptCompletion(llm LLM, prompt string, enableReasoning bool, responseChan chan StreamChunk) error {
	return llm.DoStreamPromptCompletion(prompt, enableReasoning, responseChan)
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

// BaseLLM defines fields shared by all supported LLMs
type BaseLLM struct {
	SystemPrompt string
	MaxTokens    int

	Messages    []Message
	PromptCount int
}

type Message struct {
	Role    string
	Content string
}

// Pricing defines costs per input or output token. They should be defined as `(cost per million) / 1,000,000`
type Pricing struct {
	PromptCost   float64 // per token
	ResponseCost float64 // per token
}

// BoolPtr is a helper to set optional boolean fields.
func BoolPtr(b bool) *bool {
	return &b
}

// Bubbletea messsages

// StreamChunk is the type of the channel used for communication between the API response handler and bubbletea (its also a bubbletea Msg)
type StreamChunk struct {
	Reasoning bool
	Content   string
}

type StreamError struct{ ErrMsg string }

func (e StreamError) Error() string {
	return e.ErrMsg
}
