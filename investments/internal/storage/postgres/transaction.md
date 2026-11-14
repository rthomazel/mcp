# Constants

lockInstrumentSQL = select the instrument id with FOR UPDATE.
insertTransactionSQL = insert a transactions row returning the full row.
firstNegativeSQL = for one instrument, order its ledger by effective date, execution time, id; compute the running sum of quantity_change; return the first row whose running sum is below zero, with its effective date and running sum.
latestSnapshotDateSQL = select the maximum date in snapshots, null if none.
listTransactionsSQL = select transactions matching the optional instrument, date range and event type filters, newest first by ledger order, limited.

# Functions

## Store.RecordTransaction(ctx Context, draft domain.DraftTransaction) (domain.RecordResult, error)

1. Validate draft with DraftTransaction.Validate.
2. Run inside withTx:
   1. Lock the instrument row via lockInstrumentSQL.
   2. Insert via insertTransactionSQL and scan the result.
   3. Check the whole instrument ledger with firstNegativeSQL.
   4. Read the latest snapshot date via latestSnapshotDateSQL.
   5. if the entry changes quantity and its effective date is on or before the latest snapshot date, set RebuildFrom to the effective date.
3. Log the write at info level with the transaction id, instrument id and event type.
4. Return the result.

#### Errors

- **1.** if validation fails, return the domain.ErrInvalidInput error.

---

- **2.1.** if the instrument does not exist, return domain.ErrInstrumentNotFound.
- **2.1.1.** if locking fails otherwise, return the wrapped error.

---

- **2.2.** if the insert fails a check constraint, return domain.ErrInvalidInput.
- **2.2.1.** if it fails otherwise, return the wrapped error.

---

- **2.3.** if a row comes back, return a domain.NegativeBalanceError for it; the transaction rolls back and nothing is written.
- **2.3.1.** if the query fails, return the wrapped error.

---

- **2.4.** if the query fails, return the wrapped error.

Writers for the same instrument serialize on the instrument row lock, taken before the ledger is read. Two concurrent sales therefore cannot both validate against the same balance.
Inserting first and then verifying keeps the ordering logic in one SQL query instead of reasoning about where the candidate falls in the timeline. It checks every prefix of the ledger, so a backdated sale that would make a later point negative is rejected too.
The check costs a scan of the one instrument's ledger per write, acceptable for a personal ledger.
Cash-only events never set RebuildFrom, since snapshots exclude cash.
Rebuild is never triggered here; the caller decides.

## Store.ListTransactions(ctx Context, filter domain.TransactionFilter) ([]domain.Transaction, error)

1. Normalize filter with TransactionFilter.Normalize.
2. Select via listTransactionsSQL with the filters.
3. Return the rows; empty results are an empty slice.

#### Errors

- **1.** if normalizing fails, return the domain.ErrInvalidInput error.

---

- **2.** if the query fails, return the wrapped error.
