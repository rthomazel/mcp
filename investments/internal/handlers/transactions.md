# Functions

## Handler.HandleAddTransaction(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool add_transaction. Appends one ledger entry through store.RecordTransaction. There is no tool to change or delete one.

Description shown to the agent: Append one ledger entry for a registered instrument. Signs are enforced per event type: BUY has positive quantity_change and negative cash_amount, SELL negative quantity and positive cash, DIVIDEND zero quantity and positive cash, TAX zero quantity and negative cash, SPLIT a nonzero quantity and no cash, ADJUSTMENT a nonzero quantity with optional cash. Holdings are long-only: an entry that would make ownership negative at any point in the timeline, backdated ones included, fails and writes nothing, and the error names the instrument, date and running quantity. Entries cannot be edited or deleted. Returns the stored entry. When the entry changes history that snapshots already cover, the result includes rebuild_from, the date to pass to rebuild_snapshots; recording never rebuilds on its own.

- cash_amount string, optional decimal, signed per the event type, zero when absent
- effective_date string, required YYYY-MM-DD, the date the event economically applies to; after-hours events carry the next session's date
- event_type string, required, one of BUY, SELL, DIVIDEND, TAX, SPLIT, ADJUSTMENT
- exchange string, optional when the symbol is unique
- executed_at string, required RFC 3339
- notes string, optional
- quantity_change string, required decimal, signed per the event type, zero for DIVIDEND and TAX
- symbol string, required

1. Read and parse the parameters.
2. Resolve the instrument.
3. Record the draft through store.RecordTransaction.
4. Return the transaction and, when set, rebuild_from with a note that snapshots from that date are stale until rebuild_snapshots runs.

#### Errors

- **1.** return the failure result for ErrInvalidInput.

---

- **2.** return the failure result.

---

- **3.** return the failure result; a negative balance names the instrument, the date and the quantity.

Recording never rebuilds snapshots; telling the agent when a rebuild is needed is the whole job of rebuild_from.

## Handler.HandleListTransactions(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool list_transactions. Lists ledger entries, newest first by ledger order.

Description shown to the agent: List ledger entries newest first by ledger order, optionally for one instrument and an inclusive effective-date range. Returns each entry with its symbol and exchange.

- exchange string, optional
- from string, optional YYYY-MM-DD, inclusive effective date
- limit int, optional, default 100, maximum 500
- symbol string, optional, filters to one instrument
- to string, optional YYYY-MM-DD, inclusive effective date

1. Parse the parameters and resolve the optional instrument.
2. List through store.ListTransactions.
3. Return the entries with the symbol and exchange of each.

#### Errors

- **1.** return the failure result.

---

- **2.** return the failure result.
