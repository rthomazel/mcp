# Constants

supportedVersion = 1, the only format version this build reads.
implementationTelegram = "telegram".
implementationStub = "stub".
defaultFormat = "plain".

# Types

## Config

1. Version int
2. Channels []ChannelConfig
3. Routes []Route

The complete notifications configuration, parsed from one YAML file. The handler takes it whole; no event type, producer or monitor refers to anything in it.

## ChannelConfig

1. Name string, unique, the name routes and handler names use
2. Implementation string, implementationTelegram or implementationStub
3. Categories []CategoryConfig

## CategoryConfig

1. Name string, unique within its channel
2. Settings map[string]string, the category's own settings; for Telegram, chat_id

## Route

1. EventType string
2. Format string, empty meaning defaultFormat
3. Targets []Target

## Target

1. Channel string
2. Category string

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
2. if a channel name is blank or repeated, or its implementation is unknown, return an error naming it.
3. if a category name is blank or repeated within its channel, return an error naming it.
4. if a route names an event type that is not in domain.NotifyingEvents, repeats one, or names an unknown format, return an error naming it.
5. if a target names a channel or category that is not declared, return an error naming the route and target.
6. Return nil.

A route with no targets is valid and silences its event type. A file with an unknown version is rejected whole.
