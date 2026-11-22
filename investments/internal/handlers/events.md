# Functions

## Handler.HandleRemoveEvent(ctx Context, req CallToolRequest) (*CallToolResult, error)

Tool remove_event. Removes an outbox event that was given up on or has expired.

Description shown to the agent: Remove an event from the notification outbox once its failed delivery has been dealt with, by the id get_status lists for an event given up on or expired. Only those two can be removed: an event that is pending, backing off or already handled is refused and nothing is deleted. Removing deletes the record permanently and does not resend it, and a problem that is still unresolved can be announced again by its monitor. Returns the removed id. This is the only change the agent can make to the outbox.

- id int, required, the event id from get_status

1. Parse the id.
2. Remove through store.Events.Remove.
3. Return the removed id.

#### Errors

- **1.** return the failure result for ErrInvalidInput.

---

- **2.** return the failure result; ErrEventNotFound and ErrEventNotRemovable carry their messages.

Events have no natural key, so this is the one tool that takes an internal id, exposed only through get_status.
