package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/thongntit/ghostty-config-tui/internal/schema"
)

// Model is the initial browse-only shell. Editing and persistence are added
// after the document and schema contracts stabilize.
type Model struct {
	options    []schema.Option
	selected   int
	width      int
	height     int
	configPath string
	ghostty    string
}

func NewModel(options []schema.Option, configPath, ghosttyPath string) Model {
	return Model{
		options:    options,
		configPath: configPath,
		ghostty:    ghosttyPath,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, defaultKeyMap.Quit):
			return m, tea.Quit
		case key.Matches(msg, defaultKeyMap.Up):
			if m.selected > 0 {
				m.selected--
			}
		case key.Matches(msg, defaultKeyMap.Down):
			if m.selected < len(m.options)-1 {
				m.selected++
			}
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	return view
}
