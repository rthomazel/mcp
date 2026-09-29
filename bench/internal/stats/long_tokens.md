# Long-token normalization

The package replaces long tokens with byte-counted markers.

# Types

# Functions

## pass4LongTokens(cmd) string

1. Replace each token longer than the threshold with a byte-counted marker.

## longTokenByteCount(cmd) []int

1. Collect the original byte sizes of the tokens that exceed the threshold.

#### Rationale

- Collecting byte counts before replacement preserves the information used by the stats query without retaining the content.
