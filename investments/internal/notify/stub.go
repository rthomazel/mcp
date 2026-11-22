package notify

import (
	"context"
	"log/slog"
)

// StubChannel sends nothing: it warns and reports success.
type StubChannel struct {
	name string
}

// Send logs that nothing is configured and returns nil.
func (c *StubChannel) Send(ctx context.Context, category, _ string) error {
	warnUndelivered(ctx, c.name, category)

	return nil
}

func warnUndelivered(ctx context.Context, channel, category string) {
	slog.WarnContext(ctx, "notification not delivered: nothing is configured", "channel", channel, "category", category)
}
