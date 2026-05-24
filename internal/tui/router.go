package tui

import "github.com/megalypse/go/rmqiw/internal/tui/views"

var router = map[views.ViewControl]View{
	views.MainMenu:   NewViewMainMenu(),
	views.SelectPace: NewViewSelectJourneyPace(),
	//Execution: ViewExecution{},
}
