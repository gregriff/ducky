/*
Copyright © 2025 Greg Griffin <greg.griffin2@gmail.com>
*/
package cmd

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/gregriff/ducky/config"
	"github.com/spf13/cobra"
)

var configFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ducky",
	Short: "A minimal LLM chat interface",
	Long: `ducky is a terminal-based chat interface to the LLM-provider API's (Anthropic, OpenAI).
It aims to provide a minimal feature-set with a polished UX, and supports Markdown rendering of responses.

Keybinds:
- Quit : ctrl+d
- Clear History/Quit : ctrl+c
- Toggle Focus : esc
- Text Input Controls : ctrl+a,u,k,e,n,p,b,f,h,m,t,w,d
`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	cobra.OnInitialize(func() {
		config.InitConfig(configFile)
	})

	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default is $XDG_CONFIG_HOME/ducky/ducky.toml)")
}
