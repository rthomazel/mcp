package handlers

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/rthomazel/mcp/bench/internal"
	"github.com/rthomazel/mcp/bench/internal/xml"
)

func (h *Handler) HandleShellBackground(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	commands, err := internal.ParseCommands(args, h.cfg.ShellCommandsAliases)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	cwd, _ := args["cwd"].(string)
	if cwd == "" {
		cwd = "/"
	}

	multi := len(commands) > 1
	var b xml.Builder

	b.OpenTag("metadata")

	for i, cmd := range commands {
		j := h.startJob(cmd, cwd, jobOpts{tool: "shell_background", nice: true})
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
