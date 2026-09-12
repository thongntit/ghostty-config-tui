package app

import (
	"fmt"
	"os"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/thongntit/ghostty-config-tui/internal/configdoc"
	"github.com/thongntit/ghostty-config-tui/internal/configgraph"
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

// New discovers or loads Ghostty config roots, builds an effective graph, and
// creates a dry-run editor for it. Discovery never creates files. A single
// root without includes keeps the safe scalar editor enabled; graph inputs
// with multiple files remain read-only until edit targets are explicit.
func New(options Options) (Model, error) {
	configPath := options.ConfigPath
	loadStatus := ""
	var roots []string
	if configPath == "" {
		selection, err := ghostty.DiscoverConfig()
		if err != nil {
			return Model{}, err
		}
		configPath = selection.Selected
		roots = selection.Existing
		if len(roots) == 1 {
			loadStatus = "Loaded discovered config: " + configPath
		} else {
			loadStatus = fmt.Sprintf("Loaded effective config through %s (%d default roots)", configPath, len(roots))
		}
	} else {
		loadStatus = "Loaded explicit config: " + configPath
		roots = []string{configPath}
	}

	graph, err := configgraph.Load(roots)
	if err != nil {
		return Model{}, err
	}
	document, ok := graph.DocumentFor(configPath)
	if !ok {
		return Model{}, fmt.Errorf("selected config was not loaded into graph: %q", configPath)
	}
	catalog, err := schema.EmbeddedCatalog()
	if err != nil {
		return Model{}, fmt.Errorf("load embedded Ghostty catalog: %w", err)
	}
	readOnly := true
	if len(roots) == 1 && len(graph.Includes) == 0 {
		catalog = catalog.WithBootstrapEditors()
		readOnly = false
	}
	ui := tui.NewGraphModel(optionsWithUnknowns(catalog.Options, graph), configPath, document, graph)
	ui.SetReadOnly(readOnly)
	ui.SetStatus(loadStatus)
	return Model{
		ui: ui,
	}, nil
}

func optionsWithUnknowns(options []schema.Option, graph configgraph.Graph) []schema.Option {
	result := append([]schema.Option(nil), options...)
	known := make(map[string]struct{}, len(result))
	for _, option := range result {
		known[option.Key] = struct{}{}
	}
	for _, assignment := range graph.Assignments {
		if _, exists := known[assignment.Key]; exists {
			continue
		}
		known[assignment.Key] = struct{}{}
		result = append(result, schema.Option{
			Key:         assignment.Key,
			Category:    "Custom",
			Kind:        schema.KindString,
			Description: "Unknown configuration key preserved from the loaded Ghostty config.",
			Edit:        schema.EditReadOnlyRepeatable,
		})
	}
	return result
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
