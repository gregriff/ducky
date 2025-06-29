package terminal

import (
	"os"
	"os/exec"
)

func ClearScreen() {
	var cmd *exec.Cmd
	cmd = exec.Command("clear")

	cmd.Stdout = os.Stdout
	cmd.Run()
}
