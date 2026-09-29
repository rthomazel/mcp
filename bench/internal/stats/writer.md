# Stats writer

The package records tool-call statistics to a local SQLite database.

# Types

## ToolCall

1. Tool string
2. StartedAt time.Time
3. Duration time.Duration
4. ErrorKind string
5. Command string
6. ExitCode *int
7. TimedOut bool
8. CWD string
9. JobID string
10. FilePath string
11. ReplacementCount int
12. ReplacementBytes [][2]int
13. DryRun *bool
14. Overwrite *bool
15. SetupPaths []string

## WriterConfig

1. ServerVersion string
2. EncryptionKey []byte
3. RedactPatterns []*regexp.Regexp

## Writer

1. db *sql.DB
2. ch chan ToolCall
3. cfg WriterConfig
4. wg sync.WaitGroup

# Functions

## Open(dbPath, cfg) (*Writer, error)

1. Open the database with WAL and busy timeout.
2. Run the migrations.
3. Start the background write goroutine.

## Writer.Close()

1. Close the write queue.
2. Wait up to one minute for the goroutine to drain.
3. Close the database.

## Writer.Record(tc)

1. Enqueue the tool call.
2. Drop and log when the queue is full.

## Writer.drain()

1. Insert each queued tool call.

## Writer.insert(tc)

1. Process the command through the pipeline.
2. Encrypt the normalized command when a key is present.
3. Insert the row with null-safe conversions, logging insert failures.

## Writer.QueryStats(days, bgHintThreshold) (*StatsReport, error)

1. Build the date filter and window.
2. Query the tool counts.
3. Query the top commands.
4. Return the report.

## nullString(s) any

1. Return the string or nil when empty.

## nullInt(n) any

1. Return the integer or nil when zero.

## boolInt(v) any

1. Return one, zero, or nil based on the optional boolean.

## p95(durations) *int64

1. Return nil when the sample size is below the minimum.
2. Compute the 95th percentile using the nearest-rank formula.

#### Rationale

- A single goroutine drains the write queue, so inserts are serialized and safe.
- A one-minute drain deadline prevents shutdown from hanging on a stalled queue.
