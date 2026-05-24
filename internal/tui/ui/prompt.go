package ui

import "github.com/charmbracelet/lipgloss"

func Prompt(text string) string {
	return lipgloss.NewStyle().Bold(false).Render(text)
}
