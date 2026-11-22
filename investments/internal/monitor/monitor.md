# Interfaces

## Monitor

1. Name() string
2. At() domain.ClockTime
3. Run(ctx Context) error

A job that judges a condition on the current state and records an event when it holds. Run records at most one event for each subject, and knows nothing about how events are handled. A monitor does not know how or when other jobs ran, only what state they left.

# Functions

## Task(monitor Monitor, timeout Duration) domain.ScheduledTask

Return the task named after the monitor, run daily at the monitor's At with the given timeout, whose run is the monitor's Run. The timeout is the server configuration's.

Scheduling is domain.Scheduler's: a failing monitor never stops its schedule or the other tasks, and there is no run at startup. If the server was down at the monitor's time, the next scheduled time judges the state then.
