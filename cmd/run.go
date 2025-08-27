/*
Copyright © 2025 Greg Griffin <greg.griffin2@gmail.com>
*/
package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gregriff/ducky/models"
	"github.com/gregriff/ducky/models/anthropic"
	"github.com/gregriff/ducky/models/openai"
	"github.com/gregriff/ducky/tui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"

	zone "github.com/lrstanley/bubblezone/v2"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run [model]",
	Short: "Create a new prompt session with a model",
	Long:  `Begin a prompt session with a specified model.`,
	Args:  cobra.MaximumNArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			viper.Set("model", args[0])
		}
		modelName := viper.GetString("model")
		if modelName == "" {
			return fmt.Errorf("model must be specified via argument, flag, or config file")
		}
		anthropicErr := anthropic.ValidateModelName(modelName)
		openAIErr := openai.ValidateModelName(modelName)
		if anthropicErr == nil || openAIErr == nil {
			return nil
		}

		// Neither model is valid, handle errors
		switch {
		case anthropicErr != nil && openAIErr != nil:
			// Model is neither openai nor anthropic, combine error messages
			return fmt.Errorf("Invalid model name: %s\n%v\n%v", modelName, anthropicErr, openAIErr)
		case anthropicErr != nil:
			return fmt.Errorf("Invalid model name: %s\n%v", modelName, anthropicErr)
		case openAIErr != nil:
			return fmt.Errorf("Invalid model name: %s\n%v", modelName, openAIErr)
		default:
			// This shouldn't happen if validation functions are implemented correctly
			return fmt.Errorf("Invalid model name: %s", modelName)
		}
	},
	Run: runTUI,
}

func init() {
	rootCmd.AddCommand(runCmd)

	var flagName string

	flagName = "system-prompt"
	rootCmd.PersistentFlags().StringP(flagName, "P", "", "system prompt that will influence model responses")
	viper.BindPFlag(flagName, rootCmd.PersistentFlags().Lookup(flagName))
	viper.SetDefault(flagName, "You are a concise assistant to a software engineer")

	flagName = "reasoning"
	rootCmd.PersistentFlags().BoolP(flagName, "r", true, "enable reasoning/thinking for supported models")
	viper.BindPFlag(flagName, rootCmd.PersistentFlags().Lookup(flagName))
	viper.SetDefault(flagName, true)

	flagName = "reasoning-effort"
	rootCmd.PersistentFlags().Uint8P(flagName, "e", 4, "reasoning effort to be used for specific OpenAI reasoning models (1-4) Reducing reasoning effort can result in faster responses and fewer tokens used on reasoning in a response.")
	viper.BindPFlag(flagName, rootCmd.PersistentFlags().Lookup(flagName))
	viper.SetDefault(flagName, 4)

	flagName = "max-tokens"
	rootCmd.PersistentFlags().IntP(flagName, "t", 0, "output token budget for each response")
	viper.BindPFlag(flagName, rootCmd.PersistentFlags().Lookup(flagName))
	viper.SetDefault(flagName, 2048)

	flagName = "style"
	rootCmd.PersistentFlags().StringP(flagName, "s", "", "glamour style used to render Markdown responses (default tokyo-night)")
	viper.BindPFlag(flagName, rootCmd.PersistentFlags().Lookup(flagName))
	viper.SetDefault(flagName, "tokyo-night")

	flagName = "force-interactive"
	rootCmd.PersistentFlags().Bool(flagName, false, "If stdin is a pipe, setting this option loads the TUI instead of just printing to stdout")
	viper.BindPFlag(flagName, rootCmd.PersistentFlags().Lookup(flagName))
	viper.SetDefault(flagName, false)

	flagName = "anthropic-api-key"
	rootCmd.PersistentFlags().String(flagName, "", "allows access to Claude models")
	viper.BindPFlag(flagName, rootCmd.PersistentFlags().Lookup(flagName))

	flagName = "openai-api-key"
	rootCmd.PersistentFlags().String(flagName, "", "allows access to OpenAI models")
	viper.BindPFlag(flagName, rootCmd.PersistentFlags().Lookup(flagName))
}

func runTUI(cmd *cobra.Command, args []string) {
	// note: x_API_KEY will override DUCKY_x_API_KEY here
	_, exists := os.LookupEnv("OPENAI_API_KEY")
	if !exists {
		os.Setenv("OPENAI_API_KEY", viper.GetString("openai-api-key"))
	}
	_, exists = os.LookupEnv("ANTHROPIC_API_KEY")
	if !exists {
		os.Setenv("ANTHROPIC_API_KEY", viper.GetString("anthropic-api-key"))
	}

	systemPrompt, modelName, reasoning, effort, maxTokens, style :=
		viper.GetString("system-prompt"),
		viper.GetString("model"),
		viper.GetBool("reasoning"),
		viper.GetUint8("reasoning-effort"),
		viper.GetInt("max-tokens"),
		viper.GetString("style")
	effortPtr := models.Uint8Ptr(effort)

	var initialPrompt string

	// if stdin is a pipe
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Println("error reading from stdin")
			os.Exit(1)
		}
		prompt := strings.TrimSpace(string(input))

		// if the user wants use the TUI but supply the prompt via a pipe, run app as normal
		if viper.GetBool("force-interactive") {
			initialPrompt = prompt
		} else {
			// TODO: replace this with direct calls to anthropic,openai model constructors
			model := tui.InitLLMClient(modelName, systemPrompt, maxTokens)
			responseChan := make(chan models.StreamChunk)

			var streamError error
			streamFunc := func() {
				streamError = model.DoStreamPromptCompletion(prompt, reasoning, effortPtr, responseChan)
			}
			go streamFunc()

			var fullResponse strings.Builder
			for chunk := range responseChan {
				if !chunk.Reasoning {
					fullResponse.WriteString(chunk.Content)
				}
			}
			fmt.Println(fullResponse.String())

			if streamError != nil {
				fmt.Fprintln(os.Stderr, streamError.Error())
			}
			return
		}
	}

	// Run TUI application
	zone.NewGlobal()
	tui := tui.NewTUI(
		systemPrompt,
		modelName,
		reasoning,
		effortPtr,
		maxTokens,
		style,
	)
	tui.Start(initialPrompt)
}
