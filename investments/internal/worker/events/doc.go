// Package events is the worker that drains the events table: Worker hands each
// due event whose type starts with the configured prefix to the handlers
// registered for its type and records the outcome.
package events
