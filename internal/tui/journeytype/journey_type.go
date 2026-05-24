package journeytype

type JourneyType int

const (
	TheClockIsTicking int = iota + 1
	StepByStep
)

var Journeys = []int{TheClockIsTicking, StepByStep}

var JourneyDescription = map[int]string{
	TheClockIsTicking: "The clock is ticking",
	StepByStep:        "Step by step",
}
