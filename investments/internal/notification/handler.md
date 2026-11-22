# Constants

handlerNamePrefix = "notification".
noChannelHandlerName = "notification.none".

# Types

## Source

1. loader *Loader
2. store domain.Store
3. renderers map[string]map[string]Renderer
4. telegramToken string, the bot token from the server configuration

Implements outbox.HandlerSource. It is the one notification handler, expanded for the current configuration, so the worker sees plain domain.EventHandler values.

## channelHandler

1. name string
2. renderer Renderer
3. channel Channel
4. category string

Delivers one event to one channel category: one channel and one of its categories.

## noChannelHandler

No fields.

Stands in for an event type with no notification.

# Functions

## NewSource(loader *Loader, store domain.Store, telegramToken string) *Source

Build a Source with the renderers from NewRenderers.

## Source.Handlers(ctx Context) (map[string][]domain.EventHandler, error)

1. Read the configuration from the loader.
2. Build each configured channel with newChannel, by its key.
3. For each event type of domain.NotifyingEvents, find its notification.
4. Build one channelHandler for each channel of the notification, in channel name order, for the category it names, or the channel's only category when it names none, with the renderer for the notification's format.
5. if there is no notification, use a single noChannelHandler.
6. Return the handlers by event type.

#### Errors

- **1.** if the loader fails, return the error.

Every notifying type has a handler list, so a type without a notification is handled and delivered with a warning, never an error. Channels are rebuilt on every call, which is cheap; the configuration may have changed.

## newChannel(name string, cfg ChannelConfig, telegramToken string) Channel

Return a TelegramChannel for the telegram implementation and a StubChannel otherwise. The channel's name is its implementation.

## channelHandlerName(channel string, category string) string

Return the prefix, the channel and the category joined by dots, for example notification.telegram.portfolio. The name is stored on the event, so it never changes meaning: names are not reused for another purpose and not dropped while events that name them may be in flight.

## channelHandler.Name() string

Return the stored name.

## channelHandler.Handle(ctx Context, event domain.Event) error

1. Render the event through the renderer.
2. Send the text to the category through the channel.
3. Return nil.

#### Errors

- **1.** if rendering fails, return the wrapped error.

---

- **2.** if sending fails, return the wrapped error naming the channel and category.

Channels are independent: each is its own handler with its own name on the event, so a channel that delivered is not sent again when another failed, and one failing channel does not stop the others.

## noChannelHandler.Name() string

Return noChannelHandlerName.

## noChannelHandler.Handle(ctx Context, event domain.Event) error

Log a warning naming the event type that nothing is configured to receive it and return nil.
