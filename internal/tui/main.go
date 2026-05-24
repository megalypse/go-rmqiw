package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/megalypse/go/rmqiw/internal/tui/components"
	"github.com/megalypse/go/rmqiw/internal/tui/ui"
	"github.com/megalypse/go/rmqiw/internal/tui/views"
)

func NewModel() (*RootView, error) {
	return &RootView{}, nil

}

type RootView struct {
	router tea.Model
}

func (m *RootView) Init() tea.Cmd {
	return nil
}

func (m *RootView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.router, cmd = m.router.Update(msg)

	return m, cmd
}

func (m *RootView) View() string {
	if m.router == nil {
		m.router = views.NewViewSelectJourney()
	}

	return components.NewTitle("RMQ in Wonderland") +
		ui.LineBreak +
		m.router.View()
}
