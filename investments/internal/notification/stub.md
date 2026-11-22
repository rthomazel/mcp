# Types

## StubChannel

1. name string, the configured channel name

The channel implementation that sends nothing.

# Functions

## StubChannel.SendPlainText(ctx Context, category string, text string) error

Log a warning naming the channel and category that the notification was not delivered because nothing is configured, never the text, and return nil.

Used for a channel whose implementation is stub, and by TelegramChannel for any category without a credential or chat. Every call warns, so an unconfigured system is visible in the logs.
