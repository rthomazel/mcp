package handlers

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

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

	fmt.Fprintf(&b, "tool usage (%s):\n", report.Window)

	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, ts := range report.ToolCounts {
		tfw(tw, "  %s\t%d calls", ts.Tool, ts.Count)
		if ts.Count > 0 {
			tfw(tw, "\tavg %s", msToString(ts.AvgMS))
		} else {
			tfw(tw, "\t")
		}
		if ts.P95MS != nil {
			tfw(tw, "\tp95 %s", msToString(float64(*ts.P95MS)))
		}
		tfl(tw)
	}
	_ = tw.Flush()

	if len(report.TopCommands) > 0 {
		b.WriteString("\ntop commands by frequency:\n")
		tw2 := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
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
			tfw(tw2, "  %s\t%d calls\tavg %s", label, cmd.Count, msToString(cmd.AvgMS))
			if cmd.P95MS != nil {
				tfw(tw2, "\tp95 %s", msToString(float64(*cmd.P95MS)))
			} else {
				tfw(tw2, "\t")
			}
			if cmd.HintBG {
				tfw(tw2, "\t\u2190 consider shell_background")
			}
			tfl(tw2)
		}
		_ = tw2.Flush()
	}

	if !report.HasKey {
		b.WriteString("\nnote: commands stored as hash only \u2014 configure the bench_mcp_stats_encryption_key_v1 Docker Secret to store and display full commands\n")
	}

	return b.String()
}

// tfw wraps fmt.Fprintf discarding the return values, for use with tabwriter
// where write errors are deferred to Flush.
func tfw(w *tabwriter.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, format, a...)
}

func tfl(w *tabwriter.Writer) { _, _ = fmt.Fprintln(w) }

func msToString(ms float64) string {
	if ms < 1000 {
		return fmt.Sprintf("%.1fms", ms)
	}
	return fmt.Sprintf("%.1fs", ms/1000)
}
