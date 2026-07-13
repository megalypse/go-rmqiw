package models

import "time"

type Flow struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Vars        map[string]string `json:"vars"`
	Variables   map[string]string `json:"variables"`
	Steps       []FlowStep        `json:"steps"`
}

type FlowStep struct {
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Message      Message       `json:"message"`
	PollInterval time.Duration `json:"poll_interval"`
	Timeout      time.Duration `json:"timeout"`
	PollQuery    string        `json:"poll_query"`
}
