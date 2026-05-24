package tui

type Cursor interface {
	SetOptions(options []any)
	GetOptions() []any
	IncCursor()
	DecCursor()
	GetCursor() int
	Render() string
}
