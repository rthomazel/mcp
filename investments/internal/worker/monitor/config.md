# Constants

supportedVersion = 1, the only format version this build reads.
disabledKey = "disabled", the entry key that is not a setting.

# Types

## Config

1. Version int
2. Monitors map[string]Entry, by monitor name

The monitors configuration, parsed from one YAML file by the file loader. It is separate from the notifications configuration and shares nothing with it. A custom YAML decoder:

1. Decodes the top level as a mapping and returns an error naming any key other than version and monitors.
2. Decodes the version and the entries.
3. Gives an entry written without a value an empty Entry: enabled, no settings.

## Entry

1. Disabled bool, false when omitted
2. Params Params, every key of the entry other than disabledKey

A custom YAML decoder reads the entry as a mapping of text values. disabled must be exactly true or false, otherwise it returns an error naming the key; each other key becomes a setting, and a nested value is an error. Settings that must stay exact, such as a percentage, are written as text and read by the monitor as decimals.

# Functions

## NewConfigLoader(path string, monitors []Monitor) *fileloader.Loader[Config]

Build the loader of the monitors file: fileloader.New with path and a validation that calls Config.Validate with monitors. An empty path, or a missing file, gives an empty Config, so no monitor runs.

## Config.Validate(monitors []Monitor) error

1. if Version is not supportedVersion, return an error naming the version.
2. For each entry in monitor name order, find the monitor with that name.
3. if none has it, return an error naming the entry.
4. Call the monitor's ValidateParams with the entry's Params, whether or not the entry is disabled.
5. Return nil.

#### Errors

- **3.** if no monitor has the name, return an error naming it.

---

- **4.** if the monitor rejects the settings, return the error wrapped with the monitor name.

Validation is all or nothing, as for the notifications file: the first invalid entry makes the file invalid. A mapping with a repeated key is a parse error. Whether an enabled monitor has a notification entry is not checked, the monitors file knows nothing of event routing.
