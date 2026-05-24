package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/megalypse/go/rmqiw/internal/cfg"
	"github.com/megalypse/go/rmqiw/internal/tui/ui"
	"github.com/megalypse/go/rmqiw/internal/tui/utils"
	"github.com/megalypse/go/rmqiw/internal/tui/views"
)

func NewViewMainMenu() View {
	flows, _ := cfg.GetFlows()
	flowNames := make([]string, len(flows))
	for i, flow := range flows {
		flowNames[i] = flow.Name
	}

	return ViewMainMenu{
		cursor: &DefaultCursorOptions{
			options: utils.SliceToAny(flowNames),
		},
	}
}

type ViewMainMenu struct {
	cursor Cursor
}

func (v ViewMainMenu) HandleEnter(tui *TuiInstance, selected int) {
	GlobalState.SelectedFlow = selected
	tui.SetView(views.SelectPace)
}

func (v ViewMainMenu) Update(tui *TuiInstance, msg tea.Msg) (tea.Model, tea.Cmd) {
	return UpdateDefault(tui, v.cursor, v, msg)
}

func (v ViewMainMenu) View(_ *TuiInstance) string {
	return ui.LineBreak +
		ui.MakeSubtitle("Select a hole to journey through:") +
		ui.LineSkip +
		v.cursor.Render()
}
