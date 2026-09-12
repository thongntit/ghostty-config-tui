package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/thongntit/ghostty-config-tui/internal/app"
)

func main() {
	program := tea.NewProgram(app.New())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "ghostty-config-tui:", err)
		os.Exit(1)
	}
}
