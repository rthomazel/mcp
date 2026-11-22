# Constants

heldFreshnessSQL = for each active position as of today (positionsSQL with no date), join instruments for symbol and exchange and take the maximum prices date and the days between today and it, null when it has none, ordered by symbol then exchange.
latestCompletedSnapshotSQL = select the maximum distinct prices date for which a completed snapshots run has date_from and date_to equal to it, or a completed snapshot_rebuild run has a range containing it; null when none.
attentionSQL = select every updates column for rows with status failed or incomplete that no later run supersedes, newest first by started_at then id, limited. A snapshots row is superseded when completedDateSQL is true for its date; a prices row when a later started prices run completed; a snapshot_rebuild row when a later started snapshot_rebuild run completed over a range containing its range.


The given-up and expired event queries, givenUpEventsSQL and expiredEventsSQL, are defined with the event queries in event.go.

# Functions

## Store.GetStatus(ctx Context) (domain.Status, error)

1. Derive today in Store.marketLocation.
2. Select the held instruments with their latest price date via heldFreshnessSQL.
3. Mark each instrument "missing" when it has no price date, or "stale" when its price is Store.stalePriceDays or more older than today.
4. Read the latest price date across all instruments with latestSessionDateSQL.
5. Read the latest completed snapshot date via latestCompletedSnapshotSQL.
6. if there is a latest price date, set SnapshotMissing when completedDateSQL is false for it.
7. Select the attention rows via attentionSQL.
8. Select the given-up events via givenUpEventsSQL and the expired events via expiredEventsSQL, with the store's clock as now.
9. Build the Status and call Status.Summarize.
10. Return it.

#### Errors

- **2.** if the query fails, return the wrapped error.

---

- **4.** if the query fails, return the wrapped error.

---

- **5.** if the query fails, return the wrapped error.

---

- **6.** if the query fails, return the wrapped error.

---

- **7.** if the query fails, return the wrapped error.

---

- **8.** if either query fails, return the wrapped error.

Only held instruments are checked, matching the refresh, which stops refreshing a sold instrument; a sold instrument's old prices are not a fault.
The five-day threshold already tolerates mutual funds, which publish a day later and so show a latest price a day behind.
A null error message, reason, provider or date in the attention rows becomes the empty value or nil, as in Updates.List.
Given-up events are told apart from expired ones by the event's ExpiredAt, not by when it was given up on; an event is given up on within hours of being recorded, so the two agree in practice. A given-up event therefore needs attention until its ExpiredAt and then moves, unchanged, to the expired list.
All reads are independent selects on one handle; no transaction, since each is a point-in-time fact and a concurrent run changing between them only makes the next call correct.
