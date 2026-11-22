# Types

## Params map[string]string

The settings of one monitor from its entry in the monitors configuration, as text. The monitor alone knows their meaning.

# Interfaces

## Monitor

1. Name() string
2. At() domain.ClockTime
3. ValidateParams(params Params) error
4. Run(ctx Context, params Params) error

A job that judges a condition on the current state and records an event when it holds. Run records at most one event for each subject, and knows nothing about how events are handled. A monitor does not know how or when other jobs ran, only what state they left.

### Name() string

The key of its entry in the monitors configuration. A monitor that records a notification returns its event type without domain.NotificationPrefix, for example portfolio.daily-gain for notification.portfolio.daily-gain.

### ValidateParams(params Params) error

Return an error naming any setting that is unknown, missing when required or malformed, and nil otherwise. It runs when the configuration is loaded, for disabled entries too, so Run may rely on valid settings.

# Functions

## Task(monitor Monitor, timeout Duration, loader *fileloader.Loader[Config]) domain.ScheduledTask

Return the task named after the monitor, run daily at the monitor's At with the given timeout, whose run is run for the monitor. The timeout is the server configuration's.

Scheduling is domain.Scheduler's: a failing monitor never stops its schedule or the other tasks, and there is no run at startup. If the server was down at the monitor's time, the next scheduled time judges the state then. Every monitor is scheduled whether or not it is enabled.

## run(ctx Context, monitor Monitor, loader *fileloader.Loader[Config]) error

1. Read the configuration from the loader.
2. Find the entry named monitor.Name().
3. if there is none, log at warning level that the monitor is not configured and return nil.
4. if the entry is disabled, log at info level that the monitor is disabled and return nil.
5. Return the result of monitor.Run with the entry's Params.

#### Errors

- **1.** if the loader fails, return the wrapped error.

The configuration is read on every run, so enabling, disabling and retuning need no restart. A failed read happens only when the file is invalid and no earlier valid content exists.
