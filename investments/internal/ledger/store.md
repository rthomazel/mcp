# Interfaces

## Store

1. RegisterInstrument(ctx Context, in NewInstrument) (Instrument, error)
2. ListInstruments(ctx Context) ([]Instrument, error)
3. RecordTransaction(ctx Context, in NewTransaction) (RecordResult, error)
4. ListTransactions(ctx Context, f TransactionFilter) ([]Transaction, error)

The surface consumers depend on, so the tool handlers can be tested against a mockery mock. Implemented by PostgresStore.

# Types

## PostgresStore

1. pool *pgxpool.Pool

One flat struct implementing Store; its methods are spread across instrument.go and transaction.go by concern.

# Functions

## NewPostgresStore(pool *pgxpool.Pool) *PostgresStore

Build a PostgresStore around the pool.
