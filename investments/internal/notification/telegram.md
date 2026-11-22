# Constants

telegramBaseURL = "https://api.telegram.org".
telegramTimeout = 10 seconds, the deadline for one request.
telegramMaxText = 4096 characters, the Bot API limit for one message.

# Types

## TelegramChannel

1. name string, the configured channel name
2. token string, the bot credential, empty when unset
3. chats map[string]string, the chat for each category
4. client *http.Client
5. baseURL string, telegramBaseURL, replaced in tests

The channel implementation for the Telegram Bot API, sending as the operator's bot. Each category has its own chat, a direct chat, group or broadcast channel.

# Functions

## NewTelegramChannel(cfg ChannelConfig, token string) *TelegramChannel

1. Keep the token, taken from the server configuration.
2. Collect the chat of each category from its chat_id setting.
3. Return the channel with a client using telegramTimeout.

The token comes from the server configuration, never from the notifications file.

## TelegramChannel.SendPlainText(ctx Context, category string, text string) error

1. if the token is empty or the category has no chat, log the same warning as StubChannel and return nil.
2. Truncate text to telegramMaxText characters, ending with an ellipsis when cut.
3. Post a JSON body with the chat id and the text, and no parse mode, to the sendMessage method of the bot.
4. Return nil on a success response.

#### Errors

- **3.** if the request cannot be sent, return an error built from the underlying cause without the request URL, which contains the token.
- **3.1.** if the response is not a success, return an error with the status code and Telegram's description, including when the request was rate limited; the handler fails and the event retries after its backoff.

The credential is a server-side secret: it appears in no log, error, result or event payload. Plain text means content is never rejected for formatting.
