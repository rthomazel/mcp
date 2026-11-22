package marketdata

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const runTimeout = 30 * time.Minute

// runTimes are the daily run times, ascending: 06:00 picks up mutual funds,
// 20:30 follows the equity close and corrections.
var runTimes = []domain.ClockTime{{Hour: 6}, {Hour: 20, Minute: 30}}

// RunTimesDescription renders the run times for the startup log.
func RunTimesDescription(timezone string) string {
	parts := make([]string, 0, len(runTimes))
	for _, runTime := range runTimes {
		parts = append(parts, fmt.Sprintf("%02d:%02d", runTime.Hour, runTime.Minute))
	}

	return strings.Join(parts, ", ") + " " + timezone
}

// DailyTask is the job as a task for the domain scheduler, run at runTimes.
// A failed run never stops the schedule; the next slot retries.
func DailyTask(job *Job) domain.ScheduledTask {
	return domain.ScheduledTask{
		Name: "daily job", At: runTimes, Timeout: runTimeout,
		Run: func(ctx context.Context) error {
			report, err := job.Run(ctx)
			if err != nil {
				return err
			}

			slog.InfoContext(ctx, "daily run report", "refreshed", report.Refreshed, "failed", report.Failed)

			return nil
		},
	}
}
