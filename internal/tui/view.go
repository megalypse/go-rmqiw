package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type View interface {
	Update(tui *TuiInstance, msg tea.Msg) (tea.Model, tea.Cmd)
	HandleEnter(tui *TuiInstance, selected int)
	View(tui *TuiInstance) string
}
