// Package worker runs the background workers. Notifications drains the events
// table, handing each due event to the handlers registered for its type and
// recording the outcome.
package worker
