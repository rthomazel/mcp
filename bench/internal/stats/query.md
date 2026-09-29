# Stats query

The package reads aggregated statistics from the stats database.

# Types

## StatsReport

1. Window string
2. ToolCounts []ToolStat
3. TopCommands []CmdStat
4. HasKey bool

## ToolStat

1. Tool string
2. Count int64
3. AvgMS float64
4. P95MS *int64, nil when sample size is below the minimum

## CmdStat

1. BaseCmd string
2. HashPrefix string
3. Count int64
4. AvgMS float64
5. P95MS *int64
6. Command string, decrypted redacted command, empty when no key
7. HintBG bool, p95 exceeds the background threshold

# Functions

## buildDateFilter(days) (filter, window)

1. Return an all-time filter and window when days is zero or negative.
2. Otherwise return a last-N-days filter and window.

## queryToolCounts(conn, filter) ([]ToolStat, error)

1. Aggregate the call count and average duration per tool.
2. Fetch the durations per tool for the p95.

## queryTopCommands(conn, filter, bgHintThreshold, encKey) ([]CmdStat, error)

1. Group by hash and collect the durations.
2. Sort the groups by count descending.
3. Decrypt the last encrypted value when a key is present.
4. Build the command stats, attaching the background hint when the p95 exceeds the threshold.

## fetchDurations(conn, filter, condition, arg) ([]int64, error)

1. Query the durations for the filtered rows.

#### Rationale

- Commands are grouped by hash so identical normalized commands aggregate together.
- The background hint lets the report suggest shell_background for slow commands.
