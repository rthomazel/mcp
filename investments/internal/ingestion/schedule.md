# Constants

runTimes = 06:00 and 20:30 in the market timezone (America/New_York), the two daily run times.

# Types

## Scheduler

1. job *Job
2. location *time.Location
3. clock func() Time

# Functions

## NewScheduler(job *Job) *Scheduler

Build a Scheduler in the market timezone, loaded from the embedded tzdata, using the real clock.

## nextRun(now Time, location *time.Location) Time

1. Convert now to location.
2. Return the earliest run time that is strictly after now, on that day or the next.

Built from the calendar date and wall clock in location, so daylight saving changes keep the same local times.

## Scheduler.Run(ctx Context)

1. Compute the next run time with nextRun.
2. Wait until then, or return when ctx is cancelled.
3. Run the job with a timeout, log its report at info level, or its error at error level.
4. Repeat from step 1.

A failed run never stops the loop; the next slot retries. There is no run at startup: a restart does not spend provider requests, and the next slot is at most about fourteen hours away. The job is the only caller, so runs do not overlap; the store lock also guards against an agent rebuild running at the same time.
The host timezone never matters: run times and today's date come only from the market location, never from the local zone, so a host set to another zone such as Brasilia time behaves identically. A test runs nextRun with the process zone set to a different zone to pin this.
