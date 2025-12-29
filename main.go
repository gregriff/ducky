/*
Copyright © 2025 Greg Griffin <greg.griffin2@gmail.com>
*/
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/gregriff/ducky/cmd"
)

func main() {
	if len(os.Getenv("DEBUG")) > 0 {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			fmt.Println("fatal:", err)
			panic(err)
		}
		defer func() {
			_ = f.Close()
		}()
	}
	cmd.Execute()
}
