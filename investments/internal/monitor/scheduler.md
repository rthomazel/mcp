# Constants

marketTimezone = "America/New_York", the timezone of the run times; the host timezone never matters.
monitorTimeout = 2 minutes, the deadline for one run.

# Types

## ClockTime

1. Hour int
2. Minute int

A time of day in the market timezone.

## Scheduler

1. monitors []Monitor
2. location *Location
3. clock func() Time

# Interfaces

## Monitor

1. Name() string
2. At() ClockTime
3. Run(ctx Context) error

A job that judges a condition on the current state and records an event when it holds. Run records at most one event for each subject, and knows nothing about how events are handled. A monitor does not know how or when other jobs ran, only what state they left.

# Functions

## NewScheduler(monitors []Monitor) (*Scheduler, error)

Build a Scheduler in marketTimezone loaded from the embedded tzdata, using the real clock. Returns the wrapped error when the zone cannot be loaded.

## Scheduler.Run(ctx Context)

1. Start one loop for each monitor and wait for them all.
2. In a loop, compute the next run time with nextRun and log it at info level.
3. Wait until then; if ctx is cancelled, log that the monitor stopped at info level and return.
4. Run the monitor with monitorTimeout, logging a failure at error level.
5. Repeat from step 2.

A failing monitor never stops its loop or the other monitors. There is no run at startup: if the server was down at the monitor's time, the next scheduled time judges the state then. The scheduler runs inside the server process.

## nextRun(now Time, location *Location, at ClockTime) Time

1. Convert now to location.
2. Return the earliest occurrence of at that is strictly after now, on that day or the next.

Built from the calendar date and wall clock in location, so daylight saving changes keep the same local time. It has the same rules as the daily job's nextRun.
