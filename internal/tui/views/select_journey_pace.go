package views

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/megalypse/go/rmqiw/internal/tui/components"
	"github.com/megalypse/go/rmqiw/internal/tui/ui"
)

func NewViewSelectJourneyPace(selectedFlow int) ViewSelectJourneyPace {
	options := []string{
		"The clock is ticking",
		"Step by step",
	}

	return ViewSelectJourneyPace{
		options:      components.NewListSelector(options),
		selectedFlow: selectedFlow,
	}
}

type ViewSelectJourneyPace struct {
	selectedFlow int
	options      components.ListSelector
}

func (v ViewSelectJourneyPace) Init() tea.Cmd {
	return nil
}

func (v ViewSelectJourneyPace) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			v.options.Cursor.Down()
		case "down":
			v.options.Cursor.Up()
		case "enter":
			next := NewStartJourneyContinuous(v.selectedFlow)
			return next, next.Init()
		}
	}

	return v, nil
}

func (v ViewSelectJourneyPace) View() string {
	return "At which pace?" + ui.LineSkip + v.options.Render()
}
