/*
Copyright © 2025 Greg Griffin <greg.griffin2@gmail.com>
*/
package cmd

import (
	"github.com/gregriff/gpt-cli-go/client/repl"
	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run [model to prompt]",
	Short: "Create a new prompt session with a model",
	Long:  `Begin a prompt session with a specified model, creating a new server for convenience. Server will be shutdown on exit`,
	// Args:  cobra.MinimumNArgs(1),
	Run: runRepl,
}

func init() {
	rootCmd.AddCommand(runCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// runCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// runCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func runRepl(cmd *cobra.Command, args []string) {
	repl := repl.NewREPL()
	repl.Start()
}
