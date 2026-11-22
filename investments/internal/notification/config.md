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

The complete notifications configuration, parsed from one YAML file. The handler takes it whole; no event type, producer or monitor refers to anything in it.

## ChannelConfig

1. Categories map[string]map[string]string, the settings of each category by category name; for Telegram, chat_id

Written either as a categories map or as the settings of one category given inline, which becomes the single category defaultCategory. A custom YAML decoder tells them apart: a mapping whose only key is categories is a categories map, a mapping that has categories among other keys is invalid, anything else is inline settings. An empty mapping is a channel with one category without settings. A channel written without any value never reaches that decoder, so Config has its own decoder that gives it the same single default category.

## Notification

1. Format string, required, defaultFormat
2. Channels map[string]string, the category of each channel the event is delivered to; an empty category means the channel's only category

The file holds no credentials, so it can be versioned and reviewed. Credentials are server configuration, read from environment variables into config.Config.

## Loader

1. path string
2. lastGood *Config
3. mu Mutex

Reads the file and remembers the last valid result.

# Functions

## NewLoader(path string) *Loader

Build a Loader for path. An empty path means notifications are unconfigured.

## Loader.Current() (Config, error)

1. if the path is empty or no file exists there, forget the last valid configuration and return an empty Config.
2. Read the file and parse it as YAML.
3. Validate the parsed Config.
4. Remember it as the last valid configuration and return it.

#### Errors

- **2.** if reading or parsing fails, return failed with the wrapped error.

---

- **3.** if validation fails, return failed with the error.

Failed means: log at error level, naming the path and the reason, on every call, so an invalid file is reported on every worker pass, not once. If a valid configuration was remembered, return it with a nil error so delivery carries on; otherwise return the error, and the worker fails the due events instead of treating them as delivered.
The file is read on every call. Passes are thirty seconds apart and the file is small, so caching by checksum would add a stale case for no saving. A later source, such as a database behind a dashboard, replaces the Loader and supplies the same Config.

## Config.Validate() error

1. if Version is not supportedVersion, return an error naming the version.
2. if a channel key is not a known implementation, or has an empty categories map, or a blank category name, return an error naming it.
3. if a Telegram category has no chat_id, return an error naming the category.
4. if a notification names an event type that is not in domain.NotifyingEvents, has a format other than defaultFormat, or has no channels, return an error naming it.
5. if a notification names a channel that is not declared, or a category the channel does not have, or omits the category of a channel with more than one, return an error naming the notification and channel.
6. Return nil.

An event type without a notification is not invalid: it is delivered with a warning. An entry with no channels is invalid, so silencing is removing the entry. A file with an unknown version is rejected whole. A YAML mapping with a repeated key is a parse error.
