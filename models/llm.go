/*
 * Defines fields and behavior of all supported LLMs
 */
package models

type LLM interface {
	DoStreamPromptCompletion(prompt string, enableReasoning bool, ch chan string)
	DoGetCostOfCurrentChat() float64
	DoClearChatHistory()
}

// BaseLLM defines fields shared by all supported LLMs
type BaseLLM struct {
	SystemPrompt string
	MaxTokens    uint32

	Messages    []Message
	PromptCount int
}

type Message struct {
	Role    string
	Content string
}

// TODO: handle errors
func StreamPromptCompletion(llm LLM, prompt string, enableReasoning bool, ch chan string) {
	llm.DoStreamPromptCompletion(prompt, enableReasoning, ch)
}

func GetCostOfCurrentChat(llm LLM) float64 {
	return llm.DoGetCostOfCurrentChat()
}

func ClearChatHistory(llm LLM) {
	llm.DoClearChatHistory()
}
