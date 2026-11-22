# Constants

runTimes = 06:00 and 20:30, the two daily run times in the market timezone, which is the server configuration's, shared with the monitors and with today's date; it defaults to America/New_York.
runTimeout = 30 minutes, the deadline for one daily run.

# Functions

## RunTimesDescription(timezone string) string

Render the run times for the startup log, for example the two times followed by the market timezone name.

## DailyTask(job *Job) domain.ScheduledTask

Return the task named daily job, run at runTimes with runTimeout, whose run executes the job and logs its report, refreshed and failed counts, at info level, or returns its error.

Scheduling itself is domain.Scheduler's. A failed run never stops the schedule; the next slot retries. The job is the only caller, so runs do not overlap; the store lock also guards against an agent rebuild running at the same time. There is no run at startup: a restart does not spend provider requests, and the next slot is at most about fourteen hours away.
