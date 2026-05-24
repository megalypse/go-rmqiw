package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/megalypse/go/rmqiw/internal/tui/colors"
)

type DefaultCursorOptions struct {
	options []any
	cursor  int
}

func (c *DefaultCursorOptions) Render() string {
	result := strings.Builder{}

	for i, option := range c.options {
		strOption := option.(string)
		color := colors.NeutralOption
		bold := false

		if i == c.cursor {
			color = colors.MainColor
			bold = true
		}

		result.WriteString(lipgloss.NewStyle().Bold(bold).Foreground(color).Render(strOption))
		result.WriteString("\n")
	}

	return result.String()
}

func (c *DefaultCursorOptions) SetOptions(options []any) {
	c.options = options
}

func (c *DefaultCursorOptions) GetOptions() []any {
	return c.options
}

func (c *DefaultCursorOptions) IncCursor() {
	if c.cursor < len(c.options)-1 {
		c.cursor++
	}
}

func (c *DefaultCursorOptions) DecCursor() {
	if c.cursor > 0 {
		c.cursor--
	}
}

func (c *DefaultCursorOptions) GetCursor() int {
	return c.cursor
}
