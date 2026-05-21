package handlers

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/rthomazel/bench-mcp/internal"
	"github.com/rthomazel/bench-mcp/internal/xml"
)

func (h *Handler) HandleExecBackground(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	commands, ok := internal.ParseStringSlice(req.Params.Arguments["commands"])
	if !ok || len(commands) == 0 {
		return mcp.NewToolResultError("missing required parameter: commands"), nil
	}

	cwd, _ := req.Params.Arguments["cwd"].(string)
	if cwd == "" {
		cwd = "/"
	}

	multi := len(commands) > 1
	var b xml.Builder

	b.OpenTag("metadata")

	for i, cmd := range commands {
		j := h.startJob(cmd, cwd)
		if multi {
			fmt.Fprintf(&b.Builder, "command_%d: %s\n", i, cmd)
			fmt.Fprintf(&b.Builder, "job_id_%d: %s\n", i, j.id)
		} else {
			b.WriteString("job_id: " + j.id + "\n")
		}
	}

	b.CloseTag("metadata", false)

	return mcp.NewToolResultText(b.String()), nil
}
