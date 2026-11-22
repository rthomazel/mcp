package monitor

import (
	"context"
	"time"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

// Monitor judges a condition on the current state and records an event when
// it holds, at most once for each subject.
type Monitor interface {
	Name() string
	At() domain.ClockTime
	Run(ctx context.Context) error
}

// Task is the monitor as a task for the domain scheduler, run daily at its time of day.
func Task(monitor Monitor, timeout time.Duration) domain.ScheduledTask {
	return domain.ScheduledTask{
		Name: "monitor " + monitor.Name(), At: []domain.ClockTime{monitor.At()}, Timeout: timeout, Run: monitor.Run,
	}
}
