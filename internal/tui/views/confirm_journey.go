package views

import tea "github.com/charmbracelet/bubbletea"

type ConfirmJourney struct {
	root tea.Model
}

func (c *ConfirmJourney) Init() tea.Cmd {
	return nil
}

func (c *ConfirmJourney) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "enter":
			return c, tea.Quit
		default:
			c.root = nil
		}
	}

	return c, nil
}

func (c *ConfirmJourney) View() string {
	//TODO implement me
	panic("implement me")
}
