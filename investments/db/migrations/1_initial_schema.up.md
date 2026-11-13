# Types

All five spec tables are created in the first migration so the schema is reviewed whole. Prices, snapshots and updates are used by later stages.
Money and quantities are NUMERIC without precision limits. Event types and statuses are text with CHECK constraints, not Postgres enums, so they can change in a migration without type surgery.

## instruments

1. id INTEGER GENERATED ALWAYS AS IDENTITY, primary key
2. symbol TEXT NOT NULL
3. exchange TEXT NOT NULL
4. name TEXT NOT NULL
5. currency TEXT NOT NULL
6. instrument_type TEXT NOT NULL
7. CHECK symbol and exchange are non-empty and already upper case and trimmed
8. CHECK currency = "USD"
9. CHECK instrument_type is one of "STOCK", "ETF", "FUND", "OTHER"
10. UNIQUE (symbol, exchange)

## transactions

1. id BIGINT GENERATED ALWAYS AS IDENTITY, primary key
2. instrument_id INTEGER NOT NULL, foreign key to instruments(id) ON DELETE RESTRICT
3. event_type TEXT NOT NULL
4. quantity_change NUMERIC NOT NULL
5. cash_amount NUMERIC NOT NULL DEFAULT 0
6. executed_at TIMESTAMPTZ NOT NULL
7. effective_date DATE NOT NULL
8. notes TEXT NULL
9. CHECK sign rules per event type
   1. BUY: quantity_change > 0 and cash_amount < 0
   2. SELL: quantity_change < 0 and cash_amount > 0
   3. DIVIDEND: quantity_change = 0 and cash_amount > 0
   4. TAX: quantity_change = 0 and cash_amount < 0
   5. SPLIT: quantity_change <> 0 and cash_amount = 0
   6. ADJUSTMENT: quantity_change <> 0, any cash_amount
   7. any other event_type is rejected
10. INDEX transactions_ledger_order on (instrument_id, effective_date, executed_at, id)

The index matches the chronological order of the ledger: effective date, then execution time, then identity.
Cash is optional on an adjustment, stored as zero when absent.
The check constraints duplicate the Go validation on purpose: they backstop manual administrative inserts.

## prices

1. instrument_id INTEGER NOT NULL, foreign key to instruments(id)
2. date DATE NOT NULL
3. open NUMERIC NOT NULL
4. high NUMERIC NOT NULL
5. low NUMERIC NOT NULL
6. close NUMERIC NOT NULL, CHECK close > 0
7. adjusted_close NUMERIC NOT NULL
8. volume BIGINT NOT NULL
9. PRIMARY KEY (instrument_id, date)

Column nullability is a first guess; revisit after the provider spike shows what providers actually return.

## snapshots

1. date DATE NOT NULL
2. instrument_id INTEGER NOT NULL, foreign key to instruments(id)
3. quantity NUMERIC NOT NULL, CHECK quantity > 0
4. close_price NUMERIC NOT NULL, CHECK close_price > 0
5. price_date DATE NOT NULL
6. market_value_usd NUMERIC GENERATED ALWAYS AS (quantity * close_price) STORED
7. PRIMARY KEY (date, instrument_id)

The valuation formula is enforced by the database through the generated column, so a snapshot row cannot disagree with its inputs.

## updates

1. id BIGINT GENERATED ALWAYS AS IDENTITY, primary key
2. provider TEXT NULL, null for runs that do not call a provider
3. data_type TEXT NOT NULL, one of "prices", "snapshots", "snapshot_rebuild"
4. started_at TIMESTAMPTZ NOT NULL
5. completed_at TIMESTAMPTZ NULL, null while running
6. status TEXT NOT NULL, one of "running", "completed", "incomplete", "failed"
7. records_updated INTEGER NOT NULL DEFAULT 0
8. error_message TEXT NULL
9. date_from DATE NULL
10. date_to DATE NULL
11. reason TEXT NULL
12. CHECK snapshot_rebuild rows have date_from, date_to and reason
13. INDEX updates_completed_snapshots on (date_from, date_to) WHERE data_type = "snapshots" AND status = "completed"

date_from, date_to and reason are additions to the spec's column list, needed by two spec decisions: a completed snapshot run for a date is the idempotency marker, and every rebuild is logged with its reason and range.
A daily snapshot run for date D records date_from = date_to = D. The index serves the marker lookup.
