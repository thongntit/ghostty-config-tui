package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/thongntit/ghostty-config-tui/internal/ghostty"
	"github.com/thongntit/ghostty-config-tui/internal/schema"
	"github.com/thongntit/ghostty-config-tui/internal/tui"
)

// Model composes the application services with the terminal UI. Keeping this
// boundary small lets the UI be exercised without a live Ghostty install.
type Model struct {
	ui tui.Model
}

// New creates the initial application model. It only discovers paths and the
// Ghostty binary; it does not read or modify the user's config file yet.
func New() Model {
	configPath := ghostty.ExistingConfig()
	if configPath == "" {
		candidates := ghostty.ConfigCandidates()
		if len(candidates) > 0 {
			configPath = candidates[0]
		}
	}

	return Model{
		ui: tui.NewModel(schema.BootstrapOptions(), configPath, ghostty.Find()),
	}
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
