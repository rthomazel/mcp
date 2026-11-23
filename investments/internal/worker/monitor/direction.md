# Constants

directionKey = "direction", the setting.
directionUp = "up", directionDown = "down", directionBoth = "both", the values; both when the setting is omitted.

# Types

## direction string

The side of a move a monitor watches.

# Functions

## directionFrom(params Params) (direction, error)

Read the optional setting with Params.Text, case-insensitive; both when missing; an error naming the setting for any other value.

## direction.watches(side direction) bool

Return true when the direction is both or equals side.
