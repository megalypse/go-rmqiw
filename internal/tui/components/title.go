package components

import "github.com/megalypse/go/rmqiw/internal/tui/ui"

func NewTitle(text string) string {
	return ui.Highlight(text)
}
