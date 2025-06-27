/*
 * Implements the prompt REPL used to interact with the LLMs
 */
package repl

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/gregriff/gpt-cli-go/models"
	"github.com/gregriff/gpt-cli-go/models/anthropic"
)

type REPL struct {
	model models.LLM

	scanner *bufio.Scanner
	history []string
	vars    map[string]string

	SystemPrompt string
	ModelName    string
	TotalCost    float64
	MaxTokens    int32
}

func NewREPL(systemPrompt string, modelName string, maxTokens int32) *REPL {
	return &REPL{
		scanner: bufio.NewScanner(os.Stdin),
		history: make([]string, 0),
		vars:    make(map[string]string),

		SystemPrompt: systemPrompt,
		ModelName:    modelName,
		TotalCost:    0.,
		MaxTokens:    maxTokens,
	}
}

func (r *REPL) Start() {
	// TODO: print fancy greeting
	fmt.Println("\nGPT-CLI")
	fmt.Println("Commands: :set <var> <value>, :get <var>, :history, :clear, :exit")

	model := anthropic.NewAnthropicModel(r.SystemPrompt, uint32(r.MaxTokens), r.ModelName, nil)

	for {
		// TODO: color
		fmt.Print(" > ")

		if !r.scanner.Scan() {
			break
		}

		input := strings.TrimSpace(r.scanner.Text())

		if input == "" {
			continue
		}

		if input == ":exit" || input == ":quit" {
			fmt.Println("quitting")
			break
		}

		// Add to history
		r.history = append(r.history, input)

		// Process command
		result, commandInvoked := r.processInput(input)
		if commandInvoked {
			if result != "" {
				fmt.Println(result)
			}
		} else {
			responseChan := make(chan string)
			// TODO: handle API errors here or in this func?
			go models.StreamPromptCompletion(model, input, true, responseChan)
			for resPart := range responseChan {
				fmt.Print(resPart)
			}
		}
	}
}

func (r *REPL) processInput(s string) (string, bool) {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return "", true
	}

	command := parts[0]

	// process command, prompting LLM if no command entered
	// TODO: use keybind to enter a command mode and handle seperately
	switch command {
	case ":set":
		return r.handleSet(parts), true
	case ":get":
		return r.handleGet(parts), true
	case ":history":
		return r.showHistory(), true
	case ":clear":
		r.history = r.history[:0]
		return "History cleared", true
	case ":vars":
		return r.showVars(), true
	default:
		return "", false
	}
}
