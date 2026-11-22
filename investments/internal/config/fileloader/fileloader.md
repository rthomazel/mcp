# Types

## Loader[T any]

1. path string
2. validate func(T) error
3. lastGood *T
4. mu Mutex

Reads one versioned YAML file into T and remembers the last valid result. T supplies its own YAML decoding and the validate function supplies its rules, so the loader knows nothing about notifications or monitors. Each configuration has its own Loader, so each has its own last valid content.

# Functions

## New[T any](path string, validate func(T) error) *Loader[T]

Build a Loader for path. An empty path means unconfigured.

## Loader.Current() (T, error)

1. if the path is empty or no file exists there, forget the last valid result and return the zero T.
2. Read the file and decode it as YAML into a T.
3. Validate the decoded value.
4. Remember it as the last valid result and return it.

#### Errors

- **2.** if reading or decoding fails, return failed with the wrapped error.

---

- **3.** if validation fails, return failed with the error.

Failed means: log at error level, naming the path and the reason, on every call, so an invalid file is reported on every pass or run, not once. If a valid result was remembered, return it with a nil error so work carries on; otherwise return the error, so the caller fails instead of acting on a configuration it does not have.

The file is read on every call. Callers run at most every thirty seconds and the file is small, so caching by checksum would add a stale case for no saving. A later source, such as a database behind a dashboard, replaces the Loader and supplies the same T.
