package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/megalypse/go/rmqiw/internal/tui/journeytype"
	"github.com/megalypse/go/rmqiw/internal/tui/ui"
	"github.com/megalypse/go/rmqiw/internal/tui/utils"
)

func NewViewSelectJourneyPace() View {
	options := make([]string, len(journeytype.Journeys))
	for i, journey := range journeytype.Journeys {
		options[i] = journeytype.JourneyDescription[journey]
	}

	return &ViewSelectJourneyPace{
		cursor: &DefaultCursorOptions{
			options: utils.SliceToAny(options),
		},
	}
}

type ViewSelectJourneyPace struct {
	cursor Cursor
}

func (v *ViewSelectJourneyPace) Update(tui *TuiInstance, msg tea.Msg) (tea.Model, tea.Cmd) {
	return UpdateDefault(tui, v.cursor, v, msg)
}

func (v *ViewSelectJourneyPace) HandleEnter(_ *TuiInstance, selected int) {
	GlobalState.SelectedJourney = selected
}

func (v *ViewSelectJourneyPace) View(tui *TuiInstance) string {
	return ui.LineBreak + ui.MakeSubtitle("At which pace?") + ui.LineSkip + v.cursor.Render()
}
