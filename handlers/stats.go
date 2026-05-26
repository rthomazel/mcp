package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rthomazel/bench-mcp/stats"
)

func (h *Handler) HandleStats(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.stats == nil {
		return mcp.NewToolResultText("stats are disabled (database could not be opened at startup)"), nil
	}

	days := 30
	if v, ok := req.Params.Arguments["days"]; ok && v != nil {
		if f, ok2 := v.(float64); ok2 {
			days = int(f)
		}
	}
	// 0 = all time; cap positive values to avoid accidental full-history scans
	if days < 0 {
		days = 30
	}

	threshold := h.cfg.Timeout / 2
	report, err := h.stats.QueryStats(days, threshold)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("stats query failed: %v", err)), nil
	}

	return mcp.NewToolResultText(formatStatsReport(report)), nil
}

func formatStatsReport(report *stats.StatsReport) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("tool usage (%s):\n", report.Window))

	// Column widths for alignment.
	maxTool := 4
	for _, ts := range report.ToolCounts {
		if len(ts.Tool) > maxTool {
			maxTool = len(ts.Tool)
		}
	}

	for _, ts := range report.ToolCounts {
		line := fmt.Sprintf("  %-*s  %d calls", maxTool, ts.Tool, ts.Count)
		if ts.Count > 0 {
			line += fmt.Sprintf("   avg %s", msToString(ts.AvgMS))
		}
		if ts.P95MS != nil {
			line += fmt.Sprintf("   p95 %s", msToString(float64(*ts.P95MS)))
		}
		b.WriteString(line + "\n")
	}

	if len(report.TopCommands) > 0 {
		b.WriteString("\ntop commands by frequency:\n")
		for _, cmd := range report.TopCommands {
			var label string
			if cmd.Command != "" {
				label = cmd.Command
			} else {
				baseLabel := cmd.BaseCmd
				if baseLabel == "" {
					baseLabel = "(unknown)"
				}
				label = fmt.Sprintf("%s [%s]", baseLabel, cmd.HashPrefix)
			}
			line := fmt.Sprintf("  %s   %d calls   avg %s",
				label, cmd.Count, msToString(cmd.AvgMS))
			if cmd.P95MS != nil {
				line += fmt.Sprintf("   p95 %s", msToString(float64(*cmd.P95MS)))
			}
			if cmd.HintBG {
				line += "   \u2190 consider shell_background"
			}
			b.WriteString(line + "\n")
		}
	}

	if !report.HasKey {
		b.WriteString("\nnote: commands stored as hash only \u2014 configure the bench_mcp_stats_encryption_key Docker Secret to store and display full commands\n")
	}

	return b.String()
}

func msToString(ms float64) string {
	if ms < 1000 {
		return fmt.Sprintf("%.1fms", ms)
	}
	return fmt.Sprintf("%.1fs", ms/1000)
}
