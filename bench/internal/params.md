# Internal utilities

The package holds parser helpers shared by the handlers.

# Types

# Functions

## ParseStringSlice(v) ([]string, bool)

1. Coerce the argument to a slice.
2. Return false when the value is not a slice or contains a non-string element.
3. Collect the string elements and return them.

## ParseCommands(args, aliases) ([]string, error)

1. Build the command names as commands followed by the aliases.
2. Require exactly one of them to be a non-empty string array, rejecting zero or more than one.
3. Return the parsed commands.

#### Rationale

- Exactly one source wins so the tool cannot silently choose between the canonical parameter and an alias.
