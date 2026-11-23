---
id: 2026-11-22-monitors-configuration
type: spec
summary: A versioned YAML file that turns each monitor on or off and carries its settings, loaded live by a file loader shared with the notifications configuration.
author: Thom
created: 2026-11-22
agents: claude
updated: 2026-11-23
---

# Monitors configuration

## Status

Draft. It extends the monitor mechanism of the notifications spec. The first monitor
with settings, the portfolio drawdown, has its own spec, and so does each monitor added
since. 2026-11-23 added structured settings and the instrument concept for the monitors that
watch groups and lists of instruments.

All monitors run daily at 7:00 AM Eastern. They read the state the 6:00 AM price refresh
left and do not depend on one another, so they need no order. A monitor may space its own
announcements with an interval setting, but it is still scheduled and judged daily.

## Intent

Let the operator decide which monitors run and tune what each one watches, such as a
threshold, without a code change or a restart. Monitors record events and know nothing
about how events are announced, and the notifications configuration only says where
messages go. Which monitors run and what they watch is a third concern and has its own
file, so neither configuration depends on the other.

The operator edits two small files for one feature, one that says what is watched and
one that says where it is announced. That is accepted: each file has one purpose, and
a dashboard that edits both can replace the files later.

## Concepts

**Monitor name.** The identity of a monitor in the file. For a monitor that records a
notification, it is the event type without the notification prefix: the monitor
portfolio.drawdown records notification.portfolio.drawdown. The convention keeps the
two names recognizably paired; they are separate names, and the file never mentions event
types.

**Entry.** The block of one monitor in the file: whether it is disabled, and its
settings, which belong to that monitor alone.

**Settings.** Named values a monitor reads. Each monitor defines its own and rejects any it
does not know. A value is a scalar, a list, or a list of mappings, as the monitor needs: a
list of instruments, or a list of groups each with its own keys. Numbers that must stay
exact, such as a percentage threshold or a price, are written as quoted text and read as
decimals, never as floating point. A span of time, such as a window or an interval, is a duration
written with a unit, such as 5 days or 3mo, as the duration spec defines; a bare number is invalid.
Nested keys are validated by the monitor in the same way, and an unknown key at any
depth is invalid.

**Instrument.** A monitor names an instrument by its ticker. The file loader cannot see the
database, so whether a ticker is registered is judged when the monitor runs, as a logged
warning, and never makes the file invalid. Monitors that watch several instruments may also
accept the word held, meaning every instrument with an active position.

**File loader.** The shared code that reads a versioned YAML file, validates it,
remembers the last valid result and serves it when an edit breaks the file. The monitors
configuration and the notifications configuration each use it for their own file and
share nothing else.

## Behavior

### The file

```yaml
version: 1

monitors:
  snapshot.incomplete: {}        # no value: enabled, no settings
  portfolio.drawdown:
    disabled: false              # optional, false by default
    increment_pct: "3"           # settings of this monitor
```

- `version` must be the supported version; any other value rejects the whole file.
- A file has only the keys version and monitors; any other top-level key is invalid, so a
  misspelled section can never silently leave every monitor off.
- Each key under `monitors` names a monitor the server has; an unknown name is invalid,
  including one in a disabled entry.
- An entry may be written without a value, which is an enabled entry with no settings.
- `disabled` is true or false, false when omitted. It turns a monitor off while keeping its
  settings.
- Every other key in an entry is a setting of that monitor. The monitor validates them,
  and does so for disabled entries too, so re-enabling never surfaces an old mistake.
- A mapping with a repeated key is a parse error.

### Which monitors run

A monitor runs only when its entry exists and is not disabled.

- A monitor with no entry is not configured: it does not run, and logs a warning each
  time its schedule comes up, so a missing entry is visible, as an event type without a
  notification entry is.
- A monitor with a disabled entry does not run and logs at info level.
- Removing an entry and disabling it differ only in that log level; disabling keeps the
  settings.
- With no file at all, no monitor runs.

All monitors are scheduled whether or not they are enabled, and read the file when their
time comes. Enabling, disabling and retuning take effect at the monitor's next run, with
no restart. The time of day a monitor runs stays part of the monitor and is not in the
file.

### Loading and validation

The file is read on every run, like the notifications file. The rules are the same as
there, applied to this file alone:

- Validation is all or nothing: one invalid entry makes the file invalid.
- A running file that an edit makes invalid keeps serving its last valid content, and an
  error is logged on every run while it is invalid.
- When the server starts with a monitors file that is invalid, the server does not start and
  reports the error, so a mistake such as a bad duration is found when it is made and not at the
  7:00 AM run. A path that is not set, and so no file, is not an error. The notifications file
  keeps its own rule.
- An invalid monitors file never affects the notifications file, and the reverse: each has
  its own last valid content.
- The location of each file is separate server configuration, and a path that is not set
  means no file.
- A later source, such as a database behind a dashboard, replaces the file and supplies
  the same configuration.

### Monitors and notifications are independent

A monitor records its event whether or not a notification entry routes that event type. When
there is no entry, the notification handler logs a warning and counts the event as
delivered, as the notifications spec says, and the event stays in the outbox. The
monitors file does not check the notifications file, and the notifications file does not
check which monitors exist.

### Existing monitor

The snapshot-incomplete monitor is named snapshot.incomplete and takes no settings. Because
a monitor without an entry does not run, a deployment that has no monitors file stops
announcing an incomplete snapshot until the operator adds the entry shown above.

## Non-goals

- Environment variables for monitor settings.
- Changing the time of day a monitor runs, or the market timezone, from the file.
- Settings shared by several monitors.
- Routing, formats or channels, which stay in the notifications file.
- Validating that an enabled monitor has a notification entry.
- Per-monitor last valid content within the file; validation covers the whole file.

## Decisions

- An invalid monitors file stops the server at start, so a mistake is found when it is made. A
  running server keeps its last valid content on a bad edit, because stopping a working monitor
  for a typo would be worse.
- Two files: monitors and notifications are separate concerns, each with its own version,
  last valid content and validation, so an error in one never freezes the other.
- Monitor settings are written beside `disabled` in the entry, not under a nested key,
  and a monitor owns their meaning.
- The file loader is generic and lives in its own package; each configuration provides
  its parser and its validation. It replaces the loader that was inside the notifications
  package.
- A missing entry disables a monitor, uniformly, including the first monitor. The
  alternative of keeping chosen monitors on by default was rejected because it reintroduces
  a default that lives in code.
- A missing entry warns and an explicit disabled entry does not, so only an absence the
  operator may not have intended is loud.
- Monitor names are the event type without its prefix, so the file names what is watched
  and the notifications file names what is announced.
- Unknown top-level keys are rejected in this file, because a silently ignored key would
  leave monitors off.

## Acceptance criteria

- A monitor with an enabled entry runs at its time and records its event; with no entry it
  does not run and logs a warning; with a disabled entry it does not run and logs at info
  level.
- With no monitors file, no monitor runs; adding the entry takes effect at the next run
  without a restart.
- A file with a wrong version, an unknown top-level key, an unknown monitor name, a
  non-boolean disabled, a repeated key, or settings the monitor rejects is invalid, whether
  or not the entry is disabled.
- An edit that makes a running file invalid keeps the last valid content in use and logs an
  error on every run; a monitors file that is invalid at startup stops the server with the error.
- An invalid monitors file leaves notification delivery unaffected, and an invalid
  notifications file leaves monitors unaffected.
- A monitor's event is recorded and stays in the outbox when its event type has no
  notification entry.
- Removing the disabled flag from an entry restores the monitor with its settings intact.

## Refs

- Related: 2026-11-21-notifications, the monitor mechanism and the configuration file this
  one follows.
- Related: 2026-11-21-monitor-snapshot-incomplete, the monitor that gains an entry name.
- Related: 2026-11-23-monitor-portfolio-extremes, the first monitor with settings.
- Related: 2026-11-23-monitor-allocation-drift, 2026-11-23-monitor-price-volatility,
  2026-11-23-monitor-portfolio-movement,
  2026-11-23-monitor-instrument-extremes and 2026-11-23-monitor-price-target, which use
  structured settings.
