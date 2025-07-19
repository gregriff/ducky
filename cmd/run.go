/*
Copyright © 2025 Greg Griffin <greg.griffin2@gmail.com>
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/gregriff/gpt-cli-go/models/anthropic"
	"github.com/gregriff/gpt-cli-go/tui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	zone "github.com/lrstanley/bubblezone"
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
		model := viper.GetString("model")
		if model == "" {
			return fmt.Errorf("model must be specified via argument, flag, or config file")
		}
		return anthropic.ValidateModelName(model)
	},
	Run: runTUI,
}

func init() {
	rootCmd.AddCommand(runCmd)

	rootCmd.PersistentFlags().StringP("system-prompt", "p", "", "system prompt that will influence model responses")
	viper.BindPFlag("system-prompt", rootCmd.PersistentFlags().Lookup("system-prompt"))
	viper.SetDefault("system-prompt", "You are a concise assistant to a software engineer")

	rootCmd.PersistentFlags().BoolP("reasoning", "r", true, "enable reasoning/thinking for supported models")
	viper.BindPFlag("reasoning", rootCmd.PersistentFlags().Lookup("reasoning"))
	viper.SetDefault("reasoning", true)

	rootCmd.PersistentFlags().IntP("max-tokens", "t", 0, "output token budget for each response")
	viper.BindPFlag("max-tokens", rootCmd.PersistentFlags().Lookup("max-tokens"))
	viper.SetDefault("max-tokens", 2048)

	rootCmd.PersistentFlags().StringP("style", "s", "", "glamour style used to render Markdown responses (default tokyo-night)")
	viper.BindPFlag("style", rootCmd.PersistentFlags().Lookup("style"))
	viper.SetDefault("style", "tokyo-night")

	rootCmd.PersistentFlags().String("anthropic-api-key", "", "allows access to Claude models")
	viper.BindPFlag("anthropic-api-key", rootCmd.PersistentFlags().Lookup("anthropic-api-key"))

	rootCmd.PersistentFlags().String("openai-api-key", "", "allows access to OpenAI models")
	viper.BindPFlag("openai-api-key", rootCmd.PersistentFlags().Lookup("openai-api-key"))
}

func runTUI(cmd *cobra.Command, args []string) {
	_, exists := os.LookupEnv("OPENAI_API_KEY") // TODO: check if this is the correct one
	if !exists {
		os.Setenv("OPENAI_API_KEY", viper.GetString("openai-api-key"))
	}
	_, exists = os.LookupEnv("ANTHROPIC_API_KEY")
	if !exists {
		os.Setenv("ANTHROPIC_API_KEY", viper.GetString("anthropic-api-key"))
	}

	zone.NewGlobal()
	tui := tui.NewTUI(
		viper.GetString("system-prompt"),
		viper.GetString("model"),
		viper.GetBool("reasoning"),
		viper.GetInt("max-tokens"),
		viper.GetString("style"),
	)
	tui.Start()
}
