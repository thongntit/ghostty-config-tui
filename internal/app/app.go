package app

import (
	"fmt"
	"os"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/thongntit/ghostty-config-tui/internal/configdoc"
	"github.com/thongntit/ghostty-config-tui/internal/ghostty"
	"github.com/thongntit/ghostty-config-tui/internal/schema"
	"github.com/thongntit/ghostty-config-tui/internal/tui"
)

const maxConfigSize = 1 << 20

// Options configures the config document to edit. An empty ConfigPath selects
// Ghostty's highest-precedence default config file for the current user.
type Options struct {
	ConfigPath string
}

// Model composes the loaded source document with the terminal UI. Loading is
// read-only; the TUI owns an independent draft and offers preview only.
type Model struct {
	ui tui.Model
}

// New discovers or loads one existing config file and creates a dry-run editor
// for it. Discovery never creates files or combines multiple config files.
func New(options Options) (Model, error) {
	configPath := options.ConfigPath
	loadStatus := ""
	if configPath == "" {
		selection, err := ghostty.DiscoverConfig()
		if err != nil {
			return Model{}, err
		}
		configPath = selection.Selected
		if len(selection.Existing) == 1 {
			loadStatus = "Loaded discovered config: " + configPath
		} else {
			loadStatus = fmt.Sprintf("Loaded %s (highest-precedence of %d Ghostty config files; other files are not combined)", configPath, len(selection.Existing))
		}
	} else {
		loadStatus = "Loaded explicit config: " + configPath
	}

	document, _, err := LoadConfig(configPath)
	if err != nil {
		return Model{}, err
	}
	ui := tui.NewModel(schema.BootstrapOptions(), configPath, document)
	ui.SetStatus(loadStatus)
	return Model{
		ui: ui,
	}, nil
}

// LoadConfig reads and parses an explicitly supplied regular file. It rejects
// input that is too large or not valid UTF-8 before it reaches the editor.
func LoadConfig(path string) (configdoc.Document, []byte, error) {
	if path == "" {
		return configdoc.Document{}, nil, fmt.Errorf("config path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return configdoc.Document{}, nil, fmt.Errorf("stat config %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return configdoc.Document{}, nil, fmt.Errorf("config path is not a regular file: %q", path)
	}
	if info.Size() > maxConfigSize {
		return configdoc.Document{}, nil, fmt.Errorf("config file is larger than %d bytes: %q", maxConfigSize, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return configdoc.Document{}, nil, fmt.Errorf("read config %q: %w", path, err)
	}
	if !utf8.Valid(data) {
		return configdoc.Document{}, nil, fmt.Errorf("config file is not valid UTF-8: %q", path)
	}
	return configdoc.Parse(data), data, nil
}

func (m Model) Init() tea.Cmd {
	return m.ui.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.ui.Update(msg)
	m.ui = updated
	return m, cmd
}

func (m Model) View() tea.View {
	return m.ui.View()
}
