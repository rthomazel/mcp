package notification

import "context"

// Channel delivers plain text to one category of a messaging service. SendPlainText
// returns nil only when the text was delivered.
type Channel interface {
	SendPlainText(ctx context.Context, category, text string) error
}
