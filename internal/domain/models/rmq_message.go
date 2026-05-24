package models

type Message struct {
	Headers map[string]string
	Body    []byte
}
