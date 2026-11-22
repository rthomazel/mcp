# Interfaces

## Channel

1. SendPlainText(ctx Context, category string, text string) error

The channel interface. SendPlainText delivers plain text to one category of the channel and returns nil only when it was delivered. The text carries no markup. Every notification handler depends on this and on nothing about a particular service. Implemented by TelegramChannel and StubChannel.

### SendPlainText(ctx Context, category string, text string) error

Returns an error when the message could not be delivered, which the handler reports as its own failure, so the event retries. An unknown category is treated like any category with nothing configured.
