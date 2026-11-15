-- Cash ledger for the Investments MCP. See spc/cash/spec.md.
--
-- Cash is explicit and never a security. Cash transactions are account-level
-- events with no instrument. Trades, dividends and taxes keep their cash in
-- transactions.cash_amount and are not entered here, so nothing is counted
-- twice; the balance is the sum of both from the opening balance onward.

-- Append-only-by-default, like transactions: corrections are administrative,
-- delete the bad row, insert the corrected one, then rebuild snapshots.
CREATE TABLE cash_transactions (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind           TEXT NOT NULL,
    -- Signed: positive adds cash, negative removes it.
    amount         NUMERIC NOT NULL,
    executed_at    TIMESTAMPTZ NOT NULL,
    -- Date the event economically applies to, supplied by the caller. Entries
    -- dated before the opening balance never count toward cash.
    effective_date DATE NOT NULL,
    notes          TEXT NULL,
    -- Sign rules per kind. Mirrors the Go validation on purpose, to backstop
    -- manual administrative inserts. The opening balance may be any sign,
    -- including zero. Unknown kinds match no branch and are rejected.
    CONSTRAINT cash_transactions_kind_signs CHECK (
        (kind = 'OPENING_BALANCE') OR
        (kind IN ('DEPOSIT', 'INTEREST') AND amount > 0) OR
        (kind IN ('WITHDRAWAL', 'FEE') AND amount < 0)
    )
);

-- At most one opening balance: it anchors the whole cash ledger. A partial
-- unique index on a constant allows exactly one such row and no more.
CREATE UNIQUE INDEX cash_transactions_one_opening_balance
    ON cash_transactions ((true))
    WHERE kind = 'OPENING_BALANCE';

-- Chronological ledger order, shared with the combined cash ledger query:
-- effective date, then execution time, then id.
CREATE INDEX cash_transactions_ledger_order
    ON cash_transactions (effective_date, executed_at, id);

-- Daily cash balance, one row per snapshot date, written in the same
-- transaction as that date's securities snapshot and replaced with it by a
-- rebuild. Preserved as it was, like snapshots. Kept apart from snapshots,
-- whose rows are one per instrument, so allocation stays over securities only.
-- No row exists for a date before cash was initialized.
CREATE TABLE cash_snapshots (
    date        DATE PRIMARY KEY,
    -- May be negative: reported as an integrity error, never rejected.
    balance_usd NUMERIC NOT NULL
);
