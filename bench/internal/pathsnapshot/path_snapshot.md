# Path snapshot

The package detects new binaries added to PATH by user setup scripts.

# Types

## Entry

1. Name string
2. Path string

# Functions

## Diff(home) []Entry

1. Load the snapshot, returning nil when it does not exist yet.
2. Scan the current PATH.
3. Return the entries present now but absent from the snapshot, logging other errors.

## scan() []Entry

1. Split PATH on colons.
2. Read each directory, skipping non-executable files and duplicates.
3. Sort by name then path.

## write(entries, snapshotPath) error

1. Write the entries as TSV to a temp file.
2. Flush, close, and rename over the snapshot.

## load(snapshotPath) ([]Entry, error)

1. Read the snapshot, splitting each line on tab.
2. Skip blank names.
3. Return fs.ErrNotExist when the file has not been written.

## diff(snapshot, current) []Entry

1. Walk the two sorted slices.
2. Return entries present in current but absent from snapshot.

## entryLess(a, b) bool

1. Compare by name then path.

#### Rationale

- The snapshot is written once at startup, so a diff on the first run returns nothing.
- Duplicates across PATH directories are de-duplicated so each entry is distinct.
