/*
Copyright © 2025 Greg Griffin <greg.griffin2@gmail.com>
*/
package cmd

import (
	"github.com/gregriff/gpt-cli-go/client/repl"
	"github.com/gregriff/gpt-cli-go/models/anthropic"
	"github.com/spf13/cobra"
)

var modelName string

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run [model to prompt]",
	Short: "Create a new prompt session with a model",
	Long:  `Begin a prompt session with a specified model, creating a new server for convenience. Server will be shutdown on exit`,
	// Args:  cobra.MinimumNArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return anthropic.ValidateModelName(modelName)
	},
	Run: runRepl,
}

func init() {
	rootCmd.AddCommand(runCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// runCmd.PersistentFlags().String("foo", "", "A help for foo")
	rootCmd.PersistentFlags().StringVarP(&modelName, "model", "m", "sonnet", "model to use")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// runCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func runRepl(cmd *cobra.Command, args []string) {
	systemPrompt := "You are a concise assistant to a software engineer"
	repl := repl.NewREPL(systemPrompt, modelName, 2048)
	repl.Start()
}
