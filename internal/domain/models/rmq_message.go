package models

import (
	"encoding/json"
)

type Message struct {
	Exchange   string            `json:"exchange"`
	RoutingKey string            `json:"routing_key"`
	Headers    map[string]string `json:"headers"`
	Body       json.RawMessage   `json:"body"`
}
