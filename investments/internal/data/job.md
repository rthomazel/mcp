# Types

## Job

1. store domain.Store
2. refresher *marketdata.Refresher
3. provider string, the provider name recorded in the updates log
4. clock func() Time

The daily routine, without scheduling. It holds no state beyond its dependencies.

## RunReport

1. Refreshed int, instruments refreshed without error
2. Failed int, instruments that failed or were not refreshed
3. Snapshot *domain.SnapshotResult, nil when no price data exists yet

# Functions

## NewJob(store domain.Store, refresher *marketdata.Refresher, provider string) *Job

Build a Job using the real clock.

## Job.Run(ctx Context) (RunReport, error)

1. Record the start time from clock.
2. Refresh every instrument through refresher.RefreshAll with no from date.
3. Record a prices update: completed when every instrument succeeded, incomplete otherwise, with the stored bar count, the provider name and the symbols that failed in the error message.
4. Read the latest session date through store.LatestSessionDate; if none, return the report without a snapshot.
5. Write the snapshot for that date through store.WriteSnapshot.
6. Log each step at info level: instruments refreshed and failed with the failed symbols, the prices update status, the snapshot date and its outcome (written with row count, skipped, or incomplete with the instruments and issues). Log every failure of steps 2 to 5 at error level, naming the step, and a rate limit at warn level.
7. Return the report.

#### Errors

- **2.** if listing the instruments fails, record a failed prices update, then continue with step 4.
- **2.1.** if ctx is cancelled, return the report so far and the context error without recording further.

---

- **3.** if recording fails, return the wrapped error.

---

- **4.** if reading fails, return the wrapped error.

---

- **5.** if writing fails, return the wrapped error and the report so far.

A failed or rate limited refresh does not stop the snapshot: its date is judged only by the prices stored, and stale or missing prices make it incomplete. A date already completed is skipped by the store, so repeating the run is harmless. Logs and error messages carry symbols, dates and counts only, never the credential.
