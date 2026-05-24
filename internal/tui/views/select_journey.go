package views

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/megalypse/go/rmqiw/internal/cfg"
	"github.com/megalypse/go/rmqiw/internal/tui/components"
	"github.com/megalypse/go/rmqiw/internal/tui/ui"
)

func NewViewSelectJourney() tea.Model {
	flows, _ := cfg.GetFlows()
	options := make([]string, len(flows))
	for i, flow := range flows {
		options[i] = flow.Name
	}

	return ViewSelectJourney{
		flowsList: components.NewListSelector(options),
	}
}

type ViewSelectJourney struct {
	flowsList components.ListSelector
}

func (v ViewSelectJourney) Init() tea.Cmd {
	return nil
}

func (v ViewSelectJourney) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			v.flowsList.Cursor.Down()
		case "down":
			v.flowsList.Cursor.Up()
		case "enter":
			return NewViewSelectJourneyPace(v.flowsList.Cursor.Cursor()), nil
		}
	}

	return v, nil
}

func (v ViewSelectJourney) View() string {
	return "Select a journey:" +
		ui.LineSkip +
		v.flowsList.Render()
}
