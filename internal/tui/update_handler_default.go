package tui

import tea "github.com/charmbracelet/bubbletea"

func UpdateDefault(tui *TuiInstance, cursor Cursor, view View, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return tui, tea.Quit
		case "up", "k":
			cursor.DecCursor()
		case "down", "j":
			cursor.IncCursor()
		case "enter":
			view.HandleEnter(tui, cursor.GetCursor())
		}
	}

	return tui, nil
}
