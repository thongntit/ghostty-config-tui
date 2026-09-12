package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/thongntit/ghostty-config-tui/internal/configdoc"
	"github.com/thongntit/ghostty-config-tui/internal/configgraph"
	"github.com/thongntit/ghostty-config-tui/internal/ghostty"
	"github.com/thongntit/ghostty-config-tui/internal/schema"
	"github.com/thongntit/ghostty-config-tui/internal/storage"
	"github.com/thongntit/ghostty-config-tui/internal/tui"
)

const maxConfigSize = 1 << 20

// Options configures the config document to edit. An empty ConfigPath selects
// Ghostty's highest-precedence default config file for the current user.
type Options struct {
	ConfigPath string
}

// Model composes the loaded source graph with the terminal UI. The TUI owns
// independent drafts; storage is called only after explicit confirmation.
type Model struct {
	ui tui.Model
}

// New discovers or loads Ghostty config roots, builds an effective graph, and
// creates a full editor for it. Discovery never creates files. Every loaded
// source file remains addressable and save uses a guarded multi-file commit.
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
	catalog = catalog.WithAllEditors()
	ui := tui.NewGraphModel(optionsWithUnknowns(catalog.Options, graph), configPath, document, graph)
	ghosttyBinary := ghostty.Find()
	ui.SetSaveFunc(func(changes []tui.FileSnapshot) error {
		if ghosttyBinary != "" && len(graph.Files) == 1 && len(changes) == 1 {
			if err := validateCandidate(ghosttyBinary, changes[0].Path, changes[0].Candidate); err != nil && !errors.Is(err, ghostty.ErrValidatorUnavailable) {
				return err
			}
		}
		files := make([]storage.FileChange, len(changes))
		for index, change := range changes {
			files[index] = storage.FileChange{
				Path:      change.Path,
				Original:  change.Original,
				Candidate: change.Candidate,
			}
		}
		return storage.SaveAll(files)
	})
	loadChoiceProviders(&ui)
	ui.SetStatus(loadStatus)
	return Model{
		ui: ui,
	}, nil
}

func validateCandidate(binary, sourcePath string, candidate []byte) error {
	directory := filepath.Dir(sourcePath)
	if directory == "" {
		directory = "."
	}
	prefix := ".ghostty-config-tui-validate-"
	if base := filepath.Base(sourcePath); base != "." && base != string(filepath.Separator) {
		prefix = "." + base + ".ghostty-config-tui-validate-"
	}
	temporary, err := os.CreateTemp(directory, prefix+"*.ghostty")
	if err != nil {
		return fmt.Errorf("create validation candidate: %w", err)
	}
	path := temporary.Name()
	defer os.Remove(path)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set validation candidate permissions: %w", err)
	}
	if _, err := temporary.Write(candidate); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write validation candidate: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close validation candidate: %w", err)
	}
	return ghostty.ValidateConfig(context.Background(), binary, filepath.Clean(path))
}

func loadChoiceProviders(ui *tui.Model) {
	binary := ghostty.Find()
	if binary == "" {
		ui.SetActionChoices(tui.FallbackActionChoices())
		return
	}
	var wait sync.WaitGroup
	var themes []string
	var colors []ghostty.ColorChoice
	var fonts []string
	var actions []string
	var themeErr, colorErr, fontErr, actionErr error
	wait.Add(4)
	go func() {
		defer wait.Done()
		themes, themeErr = ghostty.ListThemes(binary)
	}()
	go func() {
		defer wait.Done()
		colors, colorErr = ghostty.ListColors(binary)
	}()
	go func() {
		defer wait.Done()
		fonts, fontErr = ghostty.ListFonts(binary)
	}()
	go func() {
		defer wait.Done()
		actions, actionErr = ghostty.ListActions(binary)
	}()
	wait.Wait()
	if themeErr == nil {
		ui.SetThemeChoices(themes)
	}
	if colorErr == nil {
		choices := make([]tui.ColorChoice, len(colors))
		for index, color := range colors {
			choices[index] = tui.ColorChoice{Name: color.Name, Value: color.Value}
		}
		ui.SetColorChoices(choices)
	}
	if fontErr == nil {
		ui.SetFontChoices(fonts)
	}
	if actionErr == nil {
		ui.SetActionChoices(actions)
	}
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
			Edit:        schema.EditScalar,
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
