package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	tui "github.com/gregriff/ducky/internal"
	"github.com/gregriff/ducky/internal/models"
	"github.com/gregriff/ducky/internal/models/anthropic"
	"github.com/gregriff/ducky/internal/models/openai"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
	"golang.org/x/term"

	zone "github.com/lrstanley/bubblezone/v2"
)

// runCmd represents the run command.
var runCmd = &cobra.Command{
	Use:   "run [model]",
	Short: "Create a new prompt session with a model",
	Long:  `Begin a prompt session with a specified model.`,
	Args:  cobra.MaximumNArgs(1),
	PreRunE: func(_ *cobra.Command, args []string) error {
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
			return fmt.Errorf("invalid model name: %s\n%v\n%v", modelName, anthropicErr, openAIErr)
		case anthropicErr != nil:
			return fmt.Errorf("invalid model name: %s\n%v", modelName, anthropicErr)
		case openAIErr != nil:
			return fmt.Errorf("invalid model name: %s\n%v", modelName, openAIErr)
		default:
			// This shouldn't happen if validation functions are implemented correctly
			return fmt.Errorf("invalid model name: %s", modelName)
		}
	},
	Run: runTUI,
}

func init() {
	rootCmd.AddCommand(runCmd)

	var flagName string

	flagName = "system-prompt"
	rootCmd.PersistentFlags().StringP(flagName, "P", "", "system prompt that will influence model responses")
	_ = viper.BindPFlag(flagName, rootCmd.PersistentFlags().Lookup(flagName))
	viper.SetDefault(flagName, "You are a concise assistant to a software engineer")

	flagName = "reasoning"
	rootCmd.PersistentFlags().BoolP(flagName, "r", true, "enable reasoning/thinking for supported models")
	_ = viper.BindPFlag(flagName, rootCmd.PersistentFlags().Lookup(flagName))
	viper.SetDefault(flagName, true)

	flagName = "openai.reasoning-effort"
	rootCmd.PersistentFlags().Uint8P(flagName, "e", 4, "reasoning effort to be used for specific OpenAI reasoning models. (1-4)")
	_ = viper.BindPFlag(flagName, rootCmd.PersistentFlags().Lookup(flagName))
	viper.SetDefault(flagName, 4)

	flagName = "max-tokens"
	rootCmd.PersistentFlags().IntP(flagName, "t", 0, "output token budget for each response")
	_ = viper.BindPFlag(flagName, rootCmd.PersistentFlags().Lookup(flagName))
	viper.SetDefault(flagName, 2048)

	flagName = "style"
	rootCmd.PersistentFlags().StringP(flagName, "s", "", "glamour style used to render Markdown responses (default tokyo-night)")
	_ = viper.BindPFlag(flagName, rootCmd.PersistentFlags().Lookup(flagName))
	viper.SetDefault(flagName, "tokyo-night")

	flagName = "force-interactive"
	rootCmd.PersistentFlags().Bool(flagName, false, "if stdin is a pipe, setting this option loads the TUI instead of just printing to stdout")
	_ = viper.BindPFlag(flagName, rootCmd.PersistentFlags().Lookup(flagName))
	viper.SetDefault(flagName, false)

	flagName = "anthropic.api-key"
	rootCmd.PersistentFlags().String(flagName, "", "allows access to Claude models")
	_ = viper.BindPFlag(flagName, rootCmd.PersistentFlags().Lookup(flagName))

	flagName = "openai.api-key"
	rootCmd.PersistentFlags().String(flagName, "", "allows access to OpenAI models")
	_ = viper.BindPFlag(flagName, rootCmd.PersistentFlags().Lookup(flagName))

	flagName = "anthropic.bedrock"
	rootCmd.PersistentFlags().Bool(flagName, false, "use bedrock client (anthropic only)")
	_ = viper.BindPFlag(flagName, rootCmd.PersistentFlags().Lookup(flagName))

	flagName = "tls.extra-certs"
	rootCmd.PersistentFlags().Bool(flagName, false, "add extra root-ca certs to TLS (bedrock only)")
	_ = viper.BindPFlag(flagName, rootCmd.PersistentFlags().Lookup(flagName))

	flagName = "tls.certs-path"
	rootCmd.PersistentFlags().String(flagName, "", "path to .pem file containing extra .pem certs (bedrock only)")
	_ = viper.BindPFlag(flagName, rootCmd.PersistentFlags().Lookup(flagName))
}

func runTUI(_ *cobra.Command, _ []string) {
	// note: x_API_KEY will override DUCKY_x_API_KEY here
	_, exists := os.LookupEnv("OPENAI_API_KEY")
	if !exists {
		_ = os.Setenv("OPENAI_API_KEY", viper.GetString("openai.api-key"))
	}
	_, exists = os.LookupEnv("ANTHROPIC_API_KEY")
	if !exists {
		_ = os.Setenv("ANTHROPIC_API_KEY", viper.GetString("anthropic.api-key"))
	}

	systemPrompt, modelName, reasoning,
		effort, maxTokens, style,
		bedrockModel, extraCerts, certsPath := viper.GetString("system-prompt"),
		viper.GetString("model"),
		viper.GetBool("reasoning"),
		viper.GetUint8("openai.reasoning-effort"),
		viper.GetInt("max-tokens"),
		viper.GetString("style"),
		viper.GetBool("anthropic.bedrock"),
		viper.GetBool("tls.extra-certs"),
		viper.GetString("tls.certs-path")
	effortPtr := new(effort)

	var bedrockConfig *models.BedrockConfig
	if bedrockModel {
		c, err := models.NewBedrockConfig(extraCerts, certsPath)
		if err != nil {
			log.Fatalf("error creating bedrock config: %v", err)
		}
		bedrockConfig = &c
	}

	// TODO: replace this with direct calls to anthropic,openai model constructors
	model, err := tui.InitLLMClient(modelName, systemPrompt, maxTokens, bedrockConfig)
	if err != nil {
		fmt.Printf("error creating client for %s: %v", modelName, err)
		os.Exit(1)
	}

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
			responseChan := make(chan models.StreamChunk)

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			g, gCtx := errgroup.WithContext(ctx)
			g.Go(func() error {
				return model.StreamPromptCompletion(gCtx, prompt, reasoning, effortPtr, responseChan)
			})

			// accumulate response and print when done.
			g.Go(func() error {
				var fullResponse strings.Builder
				for {
					select {
					case <-gCtx.Done():
						return nil
					case chunk, ok := <-responseChan:
						if !ok {
							fmt.Println(fullResponse.String())
							return nil
						}
						if !chunk.Reasoning {
							fullResponse.WriteString(chunk.Content)
						}
					}
				}
			})

			// print streaming err if any.
			if err := g.Wait(); err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
			}
			return
		}
	}

	// Run TUI application
	zone.NewGlobal()
	tui := tui.NewTUI(
		model,
		systemPrompt,
		reasoning,
		effortPtr,
		maxTokens,
		style,
	)
	tui.Start(initialPrompt)
}
