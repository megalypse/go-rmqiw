package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/megalypse/go/rmqiw/internal/tui/colors"
)

func Highlight(text string) string {
	return lipgloss.NewStyle().Bold(false).Foreground(colors.MainColor).Render(text)
}
