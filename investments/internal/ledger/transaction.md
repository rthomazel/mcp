# Constants

defaultListLimit = 100.
maxListLimit = 500.

lockInstrumentSQL = select the instrument id with FOR UPDATE.
insertTransactionSQL = insert a transactions row returning the full row.
firstNegativeSQL = for one instrument, order its ledger by effective date, execution time, id; compute the running sum of quantity_change; return the first row whose running sum is below zero, with its effective date and running sum.
latestSnapshotDateSQL = select the maximum date in snapshots, null if none.
listTransactionsSQL = select transactions matching the optional filters, newest first by ledger order, limited.

# Types

## RecordResult

1. Transaction Transaction
2. RebuildFrom *Date, set when the new entry changed quantity on or before the latest existing snapshot date; snapshots from that date onward are now stale and need an explicit rebuild.

## TransactionFilter

1. InstrumentID *int
2. From *Date, inclusive effective date
3. To *Date, inclusive effective date
4. Limit int, zero means defaultListLimit, capped at maxListLimit

# Functions

## PostgresStore.RecordTransaction(ctx Context, draft DraftTransaction) (RecordResult, error)

1. Validate draft.
2. Run inside storage.WithTx:
   1. Lock the instrument row via lockInstrumentSQL.
   2. Insert via insertTransactionSQL and scan the result.
   3. Check the whole instrument ledger with firstNegativeSQL.
   4. Read the latest snapshot date via latestSnapshotDateSQL.
   5. if the entry changes quantity and its effective date is on or before the latest snapshot date, set RebuildFrom to the effective date.
3. Log the write at info level with the transaction id, instrument id and event type.
4. Return the result.

#### Errors

- **1.** if validation fails, return the ErrInvalidInput error.

---

- **2.1.** if the instrument does not exist, return ErrInstrumentNotFound.
- **2.1.1.** if locking fails otherwise, return the wrapped error.

---

- **2.2.** if the insert fails a check constraint, return ErrInvalidInput.
- **2.2.1.** if it fails otherwise, return the wrapped error.

---

- **2.3.** if a row comes back, return a NegativeBalanceError for it; the transaction rolls back and nothing is written.
- **2.3.1.** if the query fails, return the wrapped error.

---

- **2.4.** if the query fails, return the wrapped error.

Writers for the same instrument serialize on the instrument row lock, taken before the ledger is read. Two concurrent sales therefore cannot both validate against the same balance.
Inserting first and then verifying keeps the ordering logic in one SQL query instead of reasoning about where the candidate falls in the timeline. It checks every prefix of the ledger, so a backdated sale that would make a later point negative is rejected too.
The check costs a scan of the one instrument's ledger per write, acceptable for a personal ledger.
Cash-only events never set RebuildFrom, since snapshots exclude cash.
Rebuild is never triggered here; the caller decides.

## PostgresStore.ListTransactions(ctx Context, f TransactionFilter) ([]Transaction, error)

1. Apply the default and the cap to the limit.
2. Select via listTransactionsSQL with the filters.
3. Return the rows; empty results are an empty slice.

#### Errors

- **1.** if the limit is negative, return ErrInvalidInput.
- **1.1.** if From is after To, return ErrInvalidInput.

---

- **2.** if the query fails, return the wrapped error.
