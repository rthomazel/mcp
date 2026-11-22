// Package outbox is the worker that drains the events table: Notifications
// hands each due notification event to the handlers registered for its type and
// records the outcome.
package outbox
