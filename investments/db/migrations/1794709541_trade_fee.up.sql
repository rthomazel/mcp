-- Optional trade fee on buys and sells. See spc/position-pnl/spec.md.
--
-- The fee is the part of cash_amount that was a fee, never in addition to it:
-- cash counts it once, cost basis includes it on a buy and net proceeds
-- exclude it on a sell. Existing rows get zero, which keeps their meaning.
ALTER TABLE transactions
    ADD COLUMN fee NUMERIC NOT NULL DEFAULT 0;

-- Mirrors the Go validation on purpose, to backstop manual administrative
-- inserts. The bound against cash_amount lives only here, since Go does no
-- decimal arithmetic.
ALTER TABLE transactions
    ADD CONSTRAINT transactions_fee CHECK (
        fee >= 0
        AND (fee = 0 OR (event_type IN ('BUY', 'SELL') AND fee <= abs(cash_amount)))
    );
