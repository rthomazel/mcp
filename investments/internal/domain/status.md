# Types

## InstrumentStatus

1. InstrumentID int
2. Symbol string
3. Exchange string
4. LatestPriceDate *Date, nil when the instrument has no stored price
5. PriceIssue string, empty when fresh, otherwise "missing" or "stale"

Price freshness of one held instrument. Stale uses the same threshold as the portfolio: the latest price is the configured number of days or more older than the reference date.

## Status

1. Date Date, the reference date, today in the market timezone
2. Healthy bool
3. Reasons []string, one line for each thing that makes it unhealthy, empty when healthy
4. Instruments []InstrumentStatus, held instruments only, ordered by symbol then exchange
5. LatestPriceDate *Date, the most recent price date across all instruments, the date the price refresh job snapshots; nil when no prices exist
6. LatestSnapshotDate *Date, the latest price date covered by a completed snapshot run or rebuild; nil when none
7. SnapshotMissing bool, true when LatestPriceDate has no completed snapshot
8. NeedsAttention []Update, failed or incomplete runs not yet superseded, newest first, limited to defaultListLimit
9. GivenUpEvents []Event, outbox events given up on and not yet expired, newest first, limited to defaultListLimit
10. ExpiredEvents []Event, outbox events not fully handled and expired, including ones given up on earlier, newest first, limited to defaultListLimit

The health view. The raw updates log stays available through ListUpdates. A snapshot missing for a price date that was just fetched is reported as a fact, not hidden: it is expected until the next price refresh run or a rebuild.

# Functions

## Status.Summarize()

1. Add a reason for each instrument with a price issue, naming the symbol, the issue and the latest price date when there is one.
2. if SnapshotMissing, add a reason naming LatestPriceDate.
3. if NeedsAttention is not empty, add a reason with the number of unresolved runs.
4. if GivenUpEvents is not empty, add a reason with the number of events given up on.
5. Set Healthy when there are no reasons.

Pure and deterministic; the store fills the facts and calls it last. ExpiredEvents add no reason: they are listed for visibility and never change the verdict.
