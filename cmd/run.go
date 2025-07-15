/*
Copyright © 2025 Greg Griffin <greg.griffin2@gmail.com>
*/
package cmd

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/gregriff/gpt-cli-go/models/anthropic"
	"github.com/gregriff/gpt-cli-go/tui"
	"github.com/spf13/cobra"

	zone "github.com/lrstanley/bubblezone"
)

var (
	modelName       string
	enableReasoning bool
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run [model to prompt]",
	Short: "Create a new prompt session with a model",
	Long:  `Begin a prompt session with a specified model.`,
	// Args:  cobra.MinimumNArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return anthropic.ValidateModelName(modelName)
	},
	Run: runTUI,
}

func init() {
	rootCmd.AddCommand(runCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// runCmd.PersistentFlags().String("foo", "", "A help for foo")
	rootCmd.PersistentFlags().StringVarP(&modelName, "model", "m", "sonnet", "model to use")
	rootCmd.PersistentFlags().BoolVarP(&enableReasoning, "reasoning", "r", true, "enable reasoning/thinking for supported models")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// runCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func runTUI(cmd *cobra.Command, args []string) {
	systemPrompt := "You are a concise assistant to a software engineer"

	// https://github.com/charmbracelet/glamour/issues/405#issuecomment-2741476242
	glamourStyle := "light"
	if lipgloss.HasDarkBackground() {
		glamourStyle = "dark"
	}

	zone.NewGlobal()
	tui := tui.NewTUI(systemPrompt, modelName, enableReasoning, 2048, glamourStyle)
	tui.Start()
}
