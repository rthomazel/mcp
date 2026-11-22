# Functions

## Handler.HandleAddCashTransaction(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool add_cash_transaction. Appends one account-level cash entry through store.Cash.Record. There is no tool to change or delete one.

Description shown to the agent: Append one cash entry that belongs to no instrument: the opening balance, a deposit, a withdrawal, interest or an account fee. Signs are enforced per kind: OPENING_BALANCE any sign, DEPOSIT and INTEREST positive, WITHDRAWAL and FEE negative. Only one opening balance can exist, and cash is reported only from its effective date onward, so record it first. Trades, dividends and taxes are not entered here; their cash impact is counted from add_transaction. Entries cannot be edited or deleted. A balance that goes negative is recorded and reported as an integrity error, never rejected. Returns the stored entry. When the entry changes a balance that cash snapshots already cover, the result includes rebuild_from, the date to pass to rebuild_snapshots; recording never rebuilds on its own.

- amount string, required decimal, signed per the kind
- effective_date string, required YYYY-MM-DD, the date the event economically applies to
- executed_at string, required RFC 3339
- kind string, required, one of OPENING_BALANCE, DEPOSIT, WITHDRAWAL, INTEREST, FEE
- notes string, optional

1. Read and parse the parameters.
2. Record the draft through store.Cash.Record.
3. Return the entry and, when set, rebuild_from with a note that cash snapshots from that date are stale until rebuild_snapshots runs.

#### Errors

- **1.** return the failure result for ErrInvalidInput.

---

- **2.** return the failure result; a second opening balance is an ErrInvalidInput that says so.

Recording never rebuilds snapshots; telling the agent when a rebuild is needed is the whole job of rebuild_from.

## Handler.HandleListCashTransactions(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool list_cash_transactions. Lists cash entries, newest first by effective date.

Description shown to the agent: List cash entries, not trades, dividends or taxes, newest first by effective date, optionally for one kind and an inclusive effective-date range. Use list_transactions for the instrument ledger.

- from string, optional YYYY-MM-DD, inclusive effective date
- kind string, optional, one of OPENING_BALANCE, DEPOSIT, WITHDRAWAL, INTEREST, FEE
- limit int, optional, default 100, maximum 500
- to string, optional YYYY-MM-DD, inclusive effective date

1. Parse the parameters.
2. List through store.Cash.List.
3. Return the entries.

#### Errors

- **1.** return the failure result.

---

- **2.** return the failure result.

## Handler.HandleGetCash(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool get_cash. Reports the cash balance and buying power as of a date.

Description shown to the agent: Report the cash balance and buying power as of a date, default today in the New York market timezone. Cash is the opening balance plus cash entries and the cash impact of trades, dividends and taxes from the opening balance date onward; buying power is the balance floored at zero, there is no margin. Without an opening balance on or before the date, cash is reported as not initialized, with no balance or buying power, and the opening date when one exists later: record one with add_cash_transaction first. Integrity errors list any date the running balance went negative; report those to the user, they are never omitted. Securities value and account equity are in get_portfolio.

- date string, optional YYYY-MM-DD; today in the market timezone when absent

1. Parse date.
2. Read through store.Cash.Get.
3. Return the balance, with a hint to record an opening balance when it is not initialized.

#### Errors

- **1.** return the failure result.

---

- **2.** return the failure result.
