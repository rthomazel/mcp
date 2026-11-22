# Interfaces

## Renderer

1. Render(ctx Context, event domain.Event) (string, error)

Turns an event into plain, channel-neutral text, with no markup, so no handler is written for one service's formatting. A renderer may read current system state. Renderers are registered per event type and format; defaultFormat, plain, is the only format today.

# Types

## SnapshotIncompleteRenderer

1. store domain.Store

## NotificationFailedRenderer

No fields.

# Functions

## NewRenderers(store domain.Store) map[string]map[string]Renderer

Return the renderer for each event type of domain.NotifyingEvents under defaultFormat.

## SnapshotIncompleteRenderer.Render(ctx Context, event domain.Event) (string, error)

1. Decode the event data as SnapshotIncompleteData.
2. List the incomplete snapshot runs through store.ListUpdates.
3. Pick the newest whose date range is the event's date.
4. if one exists, return a message naming the date and the run's error message, which names the instruments and their issues.
5. Otherwise return a message naming the date and saying that no run was recorded for it.

#### Errors

- **1.** if decoding fails, return the wrapped error.

---

- **2.** if listing fails, return the wrapped error.

Only symbols, dates and counts reach the message. The no-run wording covers a daily job that never ran.

## NotificationFailedRenderer.Render(ctx Context, event domain.Event) (string, error)

1. Decode the event data as NotificationFailedData.
2. Return a message naming the failed event's type, when it was recorded and the number of failed attempts, and saying that its delivery was given up on.

#### Errors

- **1.** if decoding fails, return the wrapped error.

It never repeats the failed event's payload.
