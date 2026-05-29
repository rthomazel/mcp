package handlers

import (
	"context"
	"strconv"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/rthomazel/bench-mcp/internal"
	"github.com/rthomazel/bench-mcp/internal/stats"
	"github.com/rthomazel/bench-mcp/internal/xml"
)

func (h *Handler) HandleStatus(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	start := time.Now()
	defer func() {
		h.record(stats.ToolCall{Tool: "status", StartedAt: start, Duration: time.Since(start)})
	}()

	ids, ok := internal.ParseStringSlice(req.Params.Arguments["job_ids"])
	if !ok || len(ids) == 0 {
		return mcp.NewToolResultError("missing required parameter: job_ids"), nil
	}

	multi := len(ids) > 1
	var b xml.Builder

	for i, id := range ids {
		if multi {
			b.OpenTag("command", "index", strconv.Itoa(i))
		}

		formatJobStatus(&b, h, id, multi)

		if multi {
			b.CloseTag("command", true)
		}
	}

	return mcp.NewToolResultText(b.String()), nil
}

func formatJobStatus(b *xml.Builder, h *Handler, id string, includeCommand bool) {
	h.mu.RLock()
	j, exists := h.jobs[id]
	h.mu.RUnlock()

	b.OpenTag("metadata")
	if !exists {
		b.WriteString("job_id: " + id + "\n")
		b.WriteString("error: job not found\n")
		b.CloseTag("metadata", false)
		return
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	if includeCommand {
		b.WriteString("command: " + j.cmd + "\n")
	}

	b.WriteString("job_id: " + j.id + "\n")
	b.WriteString("done: " + strconv.FormatBool(j.done) + "\n")

	if j.done {
		b.WriteString("exit: " + strconv.Itoa(j.exitCode) + "\n")
	}

	b.WriteString("duration: " + time.Since(j.started).Round(time.Millisecond).String() + "\n")

	if j.err != "" {
		b.WriteString("error: " + j.err + "\n")
	}

	b.CloseTag("metadata", true)
	b.Tag("stdout", j.stdout.String(), true)
	b.Tag("stderr", j.stderr.String(), false)
}
