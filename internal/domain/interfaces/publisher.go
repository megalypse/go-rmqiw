package interfaces

import "context"

type Publisher interface {
	Publish(
		ctx context.Context,
		exchange string,
		routingKey string,
		headers map[string]string,
		body []byte,
	) error
}
