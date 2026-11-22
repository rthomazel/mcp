// Package marketdata orchestrates a price provider and the store: backfill on
// registration, incremental refreshes, and the daily job that refreshes prices
// and writes snapshots on a schedule in the market timezone.
package marketdata
