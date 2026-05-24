package interfaces

import "context"

type Poller interface {
	Poll(ctx context.Context, query string) (bool, error)
}
