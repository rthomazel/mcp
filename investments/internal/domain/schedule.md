# Types

## ClockTime

1. Hour int
2. Minute int

A time of day in the market timezone, written HH:MM.

## ScheduledTask

1. Name string
2. At []ClockTime, the times of day it runs, ascending
3. Timeout Duration, the deadline for one run
4. Run func(ctx Context) error

Work that runs daily at fixed times. The price refresh job and each monitor are one task each.

## Scheduler

1. tasks []ScheduledTask
2. location *Location
3. clock func() Time

Runs every task at its times of day in the market timezone. It never uses the host's local zone.

# Functions

## ClockTime.String() string

Return the time as zero-padded HH:MM.

## ParseClockTime(text string) (ClockTime, error)

Trim spaces and read a 24 hour HH:MM time of day. Anything else, such as 7am, 24:00, seconds or an empty text, returns domain.ErrInvalidInput naming the text.

## ParseClockTimes(text string) ([]ClockTime, error)

1. Split text on commas and read each part with ParseClockTime.
2. Sort the times ascending.
3. if a time is listed twice, return domain.ErrInvalidInput naming it.
4. Return the times.

An empty text, or an empty part such as a trailing comma, is invalid, so a list has at least one time. Ascending is what ScheduledTask.At requires.

## NewScheduler(tasks []ScheduledTask, timezone string) (*Scheduler, error)

Build a Scheduler for the tasks in the given IANA timezone, the server configuration's market timezone, loaded from the embedded tzdata, using the real clock. Returns the wrapped error when the zone cannot be loaded.

## Scheduler.Run(ctx Context)

1. Start one loop for each task and wait for them all.
2. In a loop, compute the next run time with nextRun and log it at info level with the task's name.
3. Wait until then; if ctx is cancelled, log that the task stopped at info level and return.
4. Run the task with its timeout, logging the start and the elapsed time at info level and a failure at error level.
5. Repeat from step 2.

A failing task never stops its loop or the other tasks; the next slot retries. There is no run at startup: a restart does not repeat work, and if the server was down at a task's time the next slot judges the state then. The scheduler runs inside the server process, so runs of one task do not overlap.

## nextRun(now Time, location *Location, at []ClockTime) Time

1. Convert now to location.
2. Return the earliest of the times of day that is strictly after now, on that day or the next.

Built from the calendar date and wall clock in location, so daylight saving changes keep the same local times. The host timezone never matters: times and today's date come only from the market location, so a host set to another zone such as Brasilia time behaves identically; a test runs nextRun with the process zone set to a different zone to pin this.
