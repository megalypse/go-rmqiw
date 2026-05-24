package components

import (
	"strings"

	"github.com/megalypse/go/rmqiw/internal/tui/cursor"
	"github.com/megalypse/go/rmqiw/internal/tui/ui"
)

func NewListSelector(items []string) ListSelector {
	return ListSelector{
		items:  items,
		Cursor: cursor.NewCursor(len(items) - 1),
	}
}

type ListSelector struct {
	items  []string
	Cursor *cursor.Cursor
}

func (s *ListSelector) Render() string {
	render := strings.Builder{}

	for i, item := range s.items {
		if i == s.Cursor.Cursor() {
			render.WriteString(ui.Highlight(item))
		} else {
			render.WriteString(item)
		}

		render.WriteString(ui.LineBreak)
	}

	return render.String()
}
