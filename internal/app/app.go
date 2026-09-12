package app

import (
	"fmt"
	"os"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/thongntit/ghostty-config-tui/internal/configdoc"
	"github.com/thongntit/ghostty-config-tui/internal/schema"
	"github.com/thongntit/ghostty-config-tui/internal/tui"
)

const maxConfigSize = 1 << 20

// Options configures one explicitly selected config document.
type Options struct {
	ConfigPath string
}

// Model composes the loaded source document with the terminal UI. Loading is
// read-only; the TUI owns an independent draft and offers preview only.
type Model struct {
	ui tui.Model
}

// New loads one existing config file and creates a dry-run editor for it.
func New(options Options) (Model, error) {
	document, _, err := LoadConfig(options.ConfigPath)
	if err != nil {
		return Model{}, err
	}
	return Model{
		ui: tui.NewModel(schema.BootstrapOptions(), options.ConfigPath, document),
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
