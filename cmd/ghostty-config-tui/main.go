package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/thongntit/ghostty-config-tui/internal/app"
)

func main() {
	configPath := flag.String("config", "", "path to an existing Ghostty config file (overrides discovery)")
	flag.Parse()
	configFlagSet := false
	flag.Visit(func(f *flag.Flag) {
		configFlagSet = configFlagSet || f.Name == "config"
	})
	if configFlagSet && *configPath == "" {
		fmt.Fprintln(os.Stderr, "ghostty-config-tui: --config PATH cannot be empty")
		flag.PrintDefaults()
		os.Exit(2)
	}

	model, err := app.New(app.Options{ConfigPath: *configPath})
	if err != nil {
		fmt.Fprintln(os.Stderr, "ghostty-config-tui:", err)
		os.Exit(1)
	}

	program := tea.NewProgram(model)
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "ghostty-config-tui:", err)
		os.Exit(1)
	}
}
