package views

import (
	"context"
	"time"

	"github.com/megalypse/go/rmqiw/internal/domain/interfaces"
	"github.com/megalypse/go/rmqiw/internal/domain/models"
)

func runJourneyStep(
	ctx context.Context,
	step models.FlowStep,
	publisher interfaces.Publisher,
	poller interfaces.Poller,
) error {
	if err := publisher.Publish(
		ctx,
		step.Message.Exchange,
		step.Message.RoutingKey,
		step.Message.Headers,
		step.Message.Body,
	); err != nil {
		return err
	}

	ticker := time.NewTicker(step.PollInterval * time.Millisecond)
	timeout := time.NewTimer(stepTimeout(step))
	defer ticker.Stop()
	defer timeout.Stop()

	for {
		select {
		case <-timeout.C:
			return context.DeadlineExceeded
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			ok, err := poller.Poll(ctx, step.PollQuery)
			if err != nil {
				return err
			}

			if ok {
				return nil
			}
		}
	}
}

func stepTimeout(step models.FlowStep) time.Duration {
	if step.Timeout == 0 {
		return 10 * time.Second
	}

	return step.Timeout * time.Second
}
