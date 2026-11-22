package notify

import "context"

// Channel delivers plain text to one category of a messaging service. Send
// returns nil only when the text was delivered.
type Channel interface {
	Send(ctx context.Context, category, text string) error
}
