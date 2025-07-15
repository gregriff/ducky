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
}

// TODO: handle errors
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
