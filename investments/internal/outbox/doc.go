// Package outbox drains the events table: it hands each due event to the
// handlers registered for its type and records the outcome.
package outbox
