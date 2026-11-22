# Constants

insertCashTransactionSQL = insert a cash_transactions row returning the full row.
openingDateSQL = select the effective date of the opening balance row, null when none exists.
cashLedgerSQL = the cash ledger as one relation: every cash_transactions row with its amount, unioned with every transactions row whose cash_amount is not zero, each carrying effective date, execution time, a source rank (cash_transactions first) and id for ordering. Shared by the balance and integrity queries.
cashBalanceSQL = sum the cash ledger amounts whose effective date is between the opening balance date and $1 inclusive; return the sum and the sum floored at zero, null when no opening balance exists on or before $1.
cashIntegritySQL = order the cash ledger from the opening balance date through $1 by effective date, execution time, source rank, id; compute the running sum; return the first row at which it is below zero, with its effective date and running sum.
insertCashSnapshotSQL = insert a cash_snapshots row (date, balance).
latestSnapshotDateSQL = as in transaction.go, shared.
listCashTransactionsSQL = select cash_transactions matching the optional kind and date range filters, newest first by effective date, execution time, id, limited.

# Functions

## Cash.Record(ctx Context, draft domain.CashTransaction) (domain.CashRecordResult, error)

1. Validate draft with CashTransaction.Validate.
2. Run inside withTx:
   1. Insert via insertCashTransactionSQL and scan the result.
   2. Work out the stale date with staleCashFrom, passing whether the kind is the opening balance.
3. Log the write at info level with the id and kind.
4. Return the result.

#### Errors

- **1.** if validation fails, return the domain.ErrInvalidInput error.

---

- **2.1.** if the insert fails the unique index on the opening balance, return domain.ErrInvalidInput saying an opening balance is already recorded.
- **2.1.1.** if it fails a check constraint, return domain.ErrInvalidInput.
- **2.1.2.** if it fails otherwise, return the wrapped error.

---

- **2.2.** if a query fails, return the wrapped error.

A cash balance below zero is not rejected. No row lock is needed: nothing is validated against a running balance, and the single opening balance is enforced by a partial unique index.
Recording never rebuilds; RebuildFrom only tells the caller.

## Cash.List(ctx Context, filter domain.CashTransactionFilter) ([]domain.CashTransaction, error)

1. Normalize filter with CashTransactionFilter.Normalize.
2. Select via listCashTransactionsSQL.
3. Return the rows, an empty slice when none.

#### Errors

- **1.** if normalizing fails, return the domain.ErrInvalidInput error.

---

- **2.** if the query fails, return the wrapped error.

## Cash.Get(ctx Context, date *domain.Date) (domain.CashBalance, error)

1. Derive the reference date: date, or today in Store.marketLocation.
2. Return cashBalance for it on the store's database handle.

## cashBalance(ctx Context, q queryer, date domain.Date) (domain.CashBalance, error)

1. Select the opening date via openingDateSQL.
2. if there is none, return a not initialized balance with no opening date.
3. if the opening date is after date, return a not initialized balance carrying the opening date.
4. Select the balance and buying power via cashBalanceSQL.
5. Select the first negative running balance via cashIntegritySQL, as an integrity error when a row comes back.
6. Return the initialized balance.

#### Errors

- **1.** if a query fails, return the wrapped error.

---

- **4.** if the query fails, return the wrapped error.

---

- **5.** if the query fails, return the wrapped error.

Takes the query handle so snapshots and the portfolio reuse the one definition inside their own transaction, like valuedHoldings. Sums and flooring are done by Postgres, so decimals keep full NUMERIC precision.
Entries dated before the opening balance never count. A not initialized balance reports neither zero cash nor buying power.

## staleCashFrom(ctx Context, tx *sqlx.Tx, effective domain.Date, isOpening bool) (*domain.Date, error)

1. Read the latest snapshot date via latestSnapshotDateSQL.
2. if there is none or effective is after it, return nil.
3. if isOpening, return effective.
4. Read the opening date via openingDateSQL; if one exists on or before effective, return effective, otherwise nil.

Shared by Cash.Record and RecordTransaction, so every entry that moves a stored cash balance reports the same rebuild date. An entry before the opening balance never changes a balance, so it needs no rebuild.
