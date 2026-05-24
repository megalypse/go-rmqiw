package tui

type State struct {
	SelectedFlow    int
	SelectedJourney int
}

var GlobalState = State{
	SelectedFlow:    0,
	SelectedJourney: 0,
}
