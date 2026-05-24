package ui

import "github.com/charmbracelet/lipgloss"

func MakeSubtitle(text string) string {
	return lipgloss.NewStyle().Bold(true).Render(text)
}
