package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/megalypse/go/rmqiw/internal/tui/colors"
	"github.com/megalypse/go/rmqiw/internal/tui/views"
)

type TuiInstance struct {
	view View
}

func NewModel() (*TuiInstance, error) {
	return &TuiInstance{}, nil

}

func (m *TuiInstance) Init() tea.Cmd {
	return nil
}

func (m *TuiInstance) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.view.Update(m, msg)
}

func (m *TuiInstance) View() string {
	if m.view == nil {
		m.view = router[views.MainMenu]
	}

	return lipgloss.NewStyle().Bold(true).Foreground(colors.MainColor).Render("RMQ In Wonderland") + m.view.View(m)
}

func (m *TuiInstance) SetView(view views.ViewControl) {
	m.view = router[view]
}
