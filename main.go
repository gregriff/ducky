/*
Copyright © 2026 Greg Griffin <greg.griffin2@gmail.com>
*/
package main

import (
	"log"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/gregriff/ducky/cmd"
)

func main() {
	if len(os.Getenv("DEBUG")) > 0 {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			log.Fatalf("%v", err)
		}
		defer f.Close()
	} else {
		// this ensures that any print statements accidentally left in the codebase
		// do not mess with terminal output for users.
		_, _ = tea.LogToFile(os.DevNull, "")
	}
	cmd.Execute()
}
