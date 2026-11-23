# Constants

heldWord = "held", the selection word for every instrument with an active position.

# Types

## Params map[string]any

The settings of one monitor from its entry, as decoded from YAML: text, numbers, booleans, lists and mappings. The monitor alone knows their meaning; the accessors check a shape and name the setting in every error. A missing setting is reported as ok false and never as an error, so each monitor decides what is required.

## Selection

1. Held bool, every instrument with an active position
2. Tickers []string, upper case, in the order written

# Functions

## Params.Unknown(allowed ...string) error

Return an error naming the first setting, in name order, that is not allowed, and nil otherwise.

## Params.Text(key string) (text string, ok bool, err error)

Return the setting when it is text. Any other kind is an error.

## Params.Decimal(key string) (Decimal, bool, error)

Read the setting with Text, then parse it with domain.ParseDecimal. A number written without quotes is an error, since it would pass through floating point.

## Params.Duration(key string) (duration.Duration, bool, error)

1. if the setting is missing, return ok false.
2. if it is not text, fail: a YAML number is a duration with no unit.
3. Parse it with duration.Parse, naming key as the setting.

#### Errors

- **2.** return the unit-is-missing error.

## Params.DurationOr(key string, fallback string) (duration.Duration, error)

Return the setting as Params.Duration reads it, or the fallback text parsed with the key as its setting name when the setting is missing. The fallbacks are the monitors' documented defaults.

## Params.Mappings(key string) ([]Params, bool, error)

Return the setting as a non-empty list of mappings, each as Params so nested keys are checked with the same accessors. Another shape or an empty list is an error naming the setting and, for an item, its position.

## Params.Instruments(key string) (Selection, bool, error)

1. if the setting is text, accept it only when it is the word held, ignoring case, and return Held.
2. Otherwise read a non-empty list of text items.
3. Trim and upper-case each ticker.
4. Return the tickers.

#### Errors

- **1.** any other text is an error naming the setting.

---

- **2.** a scalar that is not held, a mapping, an empty list or a non-text item is an error.

---

- **3.** an empty ticker, or one listed twice, is an error.

A single instrument is written as a list of one.

## Params.list(key string) ([]any, bool, error)

Return the setting as a non-empty list; any other shape is an error.
