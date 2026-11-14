-- Initial schema for the Investments MCP. See spc/investments-mcp/spec.md.
--
-- Conventions:
--   * Money and quantities are NUMERIC without precision limits.
--   * Event types and statuses are TEXT with CHECK constraints rather than
--     Postgres enums, so they can change in a migration without type surgery.
--   * All five tables are created here so the schema is reviewed whole; prices,
--     snapshots and updates are used by later stages.
--   * Migrations are versioned with a UNIX timestamp so parallel branches do
--     not collide.

CREATE TABLE instruments (
    id              INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    symbol          TEXT NOT NULL,
    exchange        TEXT NOT NULL,
    name            TEXT NOT NULL,
    currency        TEXT NOT NULL,
    instrument_type TEXT NOT NULL,
    -- The application normalizes to trimmed upper case; this backstops
    -- manual inserts so the (symbol, exchange) uniqueness cannot be dodged.
    CONSTRAINT instruments_symbol_normalized
        CHECK (symbol <> '' AND symbol = upper(btrim(symbol))),
    CONSTRAINT instruments_exchange_normalized
        CHECK (exchange <> '' AND exchange = upper(btrim(exchange))),
    CONSTRAINT instruments_currency_usd CHECK (currency = 'USD'),
    -- First guess at the set; revisit when real instruments are registered.
    CONSTRAINT instruments_type
        CHECK (instrument_type IN ('STOCK', 'ETF', 'FUND', 'OTHER')),
    CONSTRAINT instruments_symbol_exchange_key UNIQUE (symbol, exchange)
);

-- Append-only-by-default event ledger. Corrections are administrative: delete
-- the bad row and insert the corrected one, then rebuild affected snapshots.
CREATE TABLE transactions (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    instrument_id   INTEGER NOT NULL REFERENCES instruments (id) ON DELETE RESTRICT,
    event_type      TEXT NOT NULL,
    quantity_change NUMERIC NOT NULL,
    -- Optional on ADJUSTMENT, stored as zero when absent.
    cash_amount     NUMERIC NOT NULL DEFAULT 0,
    executed_at     TIMESTAMPTZ NOT NULL,
    -- Date the event economically applies to. Supplied by the caller: there is
    -- no market calendar, so after-hours events carry the next session's date.
    effective_date  DATE NOT NULL,
    notes           TEXT NULL,
    -- Sign rules per event type. Mirrors the Go validation on purpose, to
    -- backstop manual administrative inserts. Unknown event types match no
    -- branch and are rejected.
    CONSTRAINT transactions_event_signs CHECK (
        (event_type = 'BUY'        AND quantity_change > 0 AND cash_amount < 0) OR
        (event_type = 'SELL'       AND quantity_change < 0 AND cash_amount > 0) OR
        (event_type = 'DIVIDEND'   AND quantity_change = 0 AND cash_amount > 0) OR
        (event_type = 'TAX'        AND quantity_change = 0 AND cash_amount < 0) OR
        (event_type = 'SPLIT'      AND quantity_change <> 0 AND cash_amount = 0) OR
        (event_type = 'ADJUSTMENT' AND quantity_change <> 0)
    )
);

-- Chronological ledger order: effective date, then execution time, then id.
-- Serves position sums and the running-balance check.
CREATE INDEX transactions_ledger_order
    ON transactions (instrument_id, effective_date, executed_at, id);

-- Daily prices. close is the real closing price used for valuation;
-- adjusted_close is retained for future total-return analysis and is never
-- used to value holdings. Nullability is a first guess: revisit after the
-- market data provider spike shows what providers actually return.
CREATE TABLE prices (
    instrument_id  INTEGER NOT NULL REFERENCES instruments (id),
    date           DATE NOT NULL,
    open           NUMERIC NOT NULL,
    high           NUMERIC NOT NULL,
    low            NUMERIC NOT NULL,
    close          NUMERIC NOT NULL CHECK (close > 0),
    adjusted_close NUMERIC NOT NULL,
    volume         BIGINT NOT NULL,
    PRIMARY KEY (instrument_id, date)
);

-- Daily instrument-level valuations. Preserved as they were; never
-- recalculated implicitly. A snapshot date is all-or-nothing.
CREATE TABLE snapshots (
    date             DATE NOT NULL,
    instrument_id    INTEGER NOT NULL REFERENCES instruments (id),
    quantity         NUMERIC NOT NULL CHECK (quantity > 0),
    close_price      NUMERIC NOT NULL CHECK (close_price > 0),
    price_date       DATE NOT NULL,
    -- Generated so a row cannot disagree with its inputs.
    market_value_usd NUMERIC GENERATED ALWAYS AS (quantity * close_price) STORED,
    PRIMARY KEY (date, instrument_id)
);

-- Ingestion, snapshot and rebuild history.
-- date_from, date_to and reason are additions to the spec's column list,
-- needed by two spec decisions: a completed snapshot run for a date is the
-- idempotency marker (date_from = date_to = that date), and every rebuild is
-- logged with its reason and range.
CREATE TABLE updates (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- Null for runs that do not call a provider.
    provider        TEXT NULL,
    data_type       TEXT NOT NULL,
    started_at      TIMESTAMPTZ NOT NULL,
    -- Null while running.
    completed_at    TIMESTAMPTZ NULL,
    status          TEXT NOT NULL,
    records_updated INTEGER NOT NULL DEFAULT 0,
    error_message   TEXT NULL,
    date_from       DATE NULL,
    date_to         DATE NULL,
    reason          TEXT NULL,
    CONSTRAINT updates_data_type
        CHECK (data_type IN ('prices', 'snapshots', 'snapshot_rebuild')),
    CONSTRAINT updates_status
        CHECK (status IN ('running', 'completed', 'incomplete', 'failed')),
    CONSTRAINT updates_rebuild_logged CHECK (
        data_type <> 'snapshot_rebuild'
        OR (date_from IS NOT NULL AND date_to IS NOT NULL AND reason IS NOT NULL)
    )
);

-- Serves the idempotency marker lookup for completed snapshot runs.
CREATE INDEX updates_completed_snapshots
    ON updates (date_from, date_to)
    WHERE data_type = 'snapshots' AND status = 'completed';
