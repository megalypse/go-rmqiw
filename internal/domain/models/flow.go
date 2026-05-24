package models

type Flow struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Steps       []FlowStep `json:"steps"`
}

type FlowStep struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Message     Message `json:"message"`
	PollQuery   string  `json:"poll_query"`
}
