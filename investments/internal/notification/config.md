# Constants

supportedVersion = 1, the only format version this build reads.
implementationTelegram = "telegram".
implementationStub = "stub".
defaultFormat = "plain".
defaultCategory = "default", the name of the one category of a channel written without a categories map.

# Types

## Config

1. Version int
2. Channels map[string]ChannelConfig, by implementation
3. Notifications map[string]Notification, by event type

The complete notifications configuration, parsed from one YAML file by the file loader. The handler takes it whole; no event type, producer or monitor refers to anything in it.

## ChannelConfig

1. Categories map[string]map[string]string, the settings of each category by category name; for Telegram, chat_id

Written either as a categories map or as the settings of one category given inline, which becomes the single category defaultCategory. A custom YAML decoder tells them apart: a mapping whose only key is categories is a categories map, a mapping that has categories among other keys is invalid, anything else is inline settings. An empty mapping is a channel with one category without settings. A channel written without any value never reaches that decoder, so Config has its own decoder that gives it the same single default category.

## Notification

1. Format string, required, defaultFormat
2. Channels map[string]string, the category of each channel the event is delivered to; an empty category means the channel's only category

The file holds no credentials, so it can be versioned and reviewed. Credentials are server configuration, read from environment variables into config.Config.

# Functions

## NewConfigLoader(path string) *fileloader.Loader[Config]

Build the loader of the notifications file: fileloader.New with path and Config.Validate. An empty path means notifications are unconfigured. Reading, the last valid configuration and failure logging are the file loader's; a missing file or an unset path gives an empty Config, which has no notification entries.

## Config.Validate() error

1. if Version is not supportedVersion, return an error naming the version.
2. if a channel key is not a known implementation, or has an empty categories map, or a blank category name, return an error naming it.
3. if a Telegram category has no chat_id, return an error naming the category.
4. if a notification names an event type that does not start with domain.NotificationPrefix, has a format other than defaultFormat, or has no channels, return an error naming it.
5. if a notification names a channel that is not declared, or a category the channel does not have, or omits the category of a channel with more than one, return an error naming the notification and channel.
6. Return nil.

An event type without a notification is not invalid: it is delivered with a warning. An entry with no channels is invalid, so silencing is removing the entry. A file with an unknown version is rejected whole. A YAML mapping with a repeated key is a parse error.
