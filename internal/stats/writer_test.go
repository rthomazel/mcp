package stats

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rthomazel/bench-mcp/internal"
)

// openTestWriter creates a Writer backed by a temp-file SQLite database.
// The writer is closed automatically via t.Cleanup.
func openTestWriter(t *testing.T) *Writer {
	t.Helper()
	w, err := Open(filepath.Join(t.TempDir(), "stats.db"), WriterConfig{ServerVersion: "v-test"})
	if err != nil {
		t.Fatalf("open test writer: %v", err)
	}
	t.Cleanup(w.Close)
	return w
}

// rawInsert inserts a minimal shell row directly, bypassing the pipeline.
// Useful for injecting rows with specific field values (e.g. a stale normalizer_version).
func rawInsert(t *testing.T, db *sql.DB, tool, cmdHash, baseCmd string, durationMS int64, normVer int) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO tool_calls (id, tool, called_at, duration_ms, base_cmd, cmd_hash, normalizer_version)
		 VALUES (?, ?, datetime('now'), ?, ?, ?, ?)`,
		uuid.New().String(), tool, durationMS, nullString(baseCmd), nullString(cmdHash), normVer,
	)
	if err != nil {
		t.Fatalf("rawInsert: %v", err)
	}
}

// --- tests ---

func TestQueryStats_Empty(t *testing.T) {
	w := openTestWriter(t)
	report, err := w.QueryStats(0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.ToolCounts) != 0 {
		t.Errorf("expected no tool counts, got %d", len(report.ToolCounts))
	}
	if len(report.TopCommands) != 0 {
		t.Errorf("expected no top commands, got %d", len(report.TopCommands))
	}
}

func TestQueryStats_WindowLabel(t *testing.T) {
	w := openTestWriter(t)
	cases := []struct {
		days int
		want string
	}{
		{0, "all time"},
		{7, "last 7 days"},
		{30, "last 30 days"},
	}
	for _, tc := range cases {
		report, err := w.QueryStats(tc.days, 0)
		if err != nil {
			t.Fatalf("days=%d: %v", tc.days, err)
		}
		if report.Window != tc.want {
			t.Errorf("days=%d: Window = %q, want %q", tc.days, report.Window, tc.want)
		}
	}
}

func TestQueryStats_ToolCounts(t *testing.T) {
	w := openTestWriter(t)

	// 3 shell calls at 1 s each.
	for i := 0; i < 3; i++ {
		w.insert(ToolCall{Tool: "shell", StartedAt: time.Now(), Duration: time.Second})
	}
	// 2 file_replace calls at 200 ms each.
	for i := 0; i < 2; i++ {
		w.insert(ToolCall{Tool: "file_replace", StartedAt: time.Now(), Duration: 200 * time.Millisecond})
	}

	report, err := w.QueryStats(0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.ToolCounts) != 2 {
		t.Fatalf("expected 2 tool entries, got %d", len(report.ToolCounts))
	}

	byTool := make(map[string]ToolStat)
	for _, ts := range report.ToolCounts {
		byTool[ts.Tool] = ts
	}

	shell, ok := byTool["shell"]
	if !ok {
		t.Fatal("missing shell in tool counts")
	}
	if shell.Count != 3 {
		t.Errorf("shell count = %d, want 3", shell.Count)
	}
	if shell.AvgMS != 1000 {
		t.Errorf("shell avg_ms = %.1f, want 1000", shell.AvgMS)
	}

	fr, ok := byTool["file_replace"]
	if !ok {
		t.Fatal("missing file_replace in tool counts")
	}
	if fr.Count != 2 {
		t.Errorf("file_replace count = %d, want 2", fr.Count)
	}
	if fr.AvgMS != 200 {
		t.Errorf("file_replace avg_ms = %.1f, want 200", fr.AvgMS)
	}
}

func TestQueryStats_P95(t *testing.T) {
	t.Run("nil when below min samples", func(t *testing.T) {
		w := openTestWriter(t)
		for i := 0; i < internal.StatsP95MinSamples-1; i++ {
			w.insert(ToolCall{Tool: "context", StartedAt: time.Now(), Duration: time.Second})
		}
		report, err := w.QueryStats(0, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(report.ToolCounts) == 0 {
			t.Fatal("expected tool counts")
		}
		if report.ToolCounts[0].P95MS != nil {
			t.Errorf("expected nil p95 with %d samples, got %d",
				internal.StatsP95MinSamples-1, *report.ToolCounts[0].P95MS)
		}
	})

	t.Run("computed at min samples", func(t *testing.T) {
		w := openTestWriter(t)
		// Insert StatsP95MinSamples rows with durations 1 ms, 2 ms, ..., N ms.
		n := internal.StatsP95MinSamples
		for i := 1; i <= n; i++ {
			w.insert(ToolCall{
				Tool:      "shell",
				StartedAt: time.Now(),
				Duration:  time.Duration(i) * time.Millisecond,
			})
		}
		report, err := w.QueryStats(0, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(report.ToolCounts) == 0 {
			t.Fatal("expected tool counts")
		}
		got := report.ToolCounts[0].P95MS
		if got == nil {
			t.Fatal("expected non-nil p95")
		}
		// nearest-rank: ceil(0.95 * 20) - 1 = 18 (0-based); sorted[18] = 19 ms
		const want = int64(19)
		if *got != want {
			t.Errorf("p95 = %d ms, want %d ms", *got, want)
		}
	})
}

func TestQueryStats_DateFilter(t *testing.T) {
	w := openTestWriter(t)

	// One row 31 days ago, one row now.
	w.insert(ToolCall{Tool: "shell", StartedAt: time.Now().AddDate(0, 0, -31), Duration: time.Second})
	w.insert(ToolCall{Tool: "shell", StartedAt: time.Now(), Duration: time.Second})

	// All time: both rows.
	report, err := w.QueryStats(0, 0)
	if err != nil {
		t.Fatalf("all time: %v", err)
	}
	if len(report.ToolCounts) == 0 || report.ToolCounts[0].Count != 2 {
		t.Errorf("all time: expected count=2, got %+v", report.ToolCounts)
	}

	// Last day: only the recent row.
	report, err = w.QueryStats(1, 0)
	if err != nil {
		t.Fatalf("last day: %v", err)
	}
	if len(report.ToolCounts) == 0 || report.ToolCounts[0].Count != 1 {
		t.Errorf("last day: expected count=1, got %+v", report.ToolCounts)
	}
}

func TestQueryStats_TopCommandsSortedByCount(t *testing.T) {
	w := openTestWriter(t)

	// "go test" 5 times, "git status" 3 times.
	for i := 0; i < 5; i++ {
		w.insert(ToolCall{Tool: "shell", StartedAt: time.Now(), Duration: time.Second, Command: "go test ./..."})
	}
	for i := 0; i < 3; i++ {
		w.insert(ToolCall{Tool: "shell", StartedAt: time.Now(), Duration: 500 * time.Millisecond, Command: "git status"})
	}

	report, err := w.QueryStats(0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.TopCommands) < 2 {
		t.Fatalf("expected >=2 top commands, got %d", len(report.TopCommands))
	}
	if report.TopCommands[0].Count < report.TopCommands[1].Count {
		t.Errorf("not sorted: [0].Count=%d < [1].Count=%d",
			report.TopCommands[0].Count, report.TopCommands[1].Count)
	}
	if report.TopCommands[0].Count != 5 {
		t.Errorf("top command count = %d, want 5", report.TopCommands[0].Count)
	}
	if report.TopCommands[0].BaseCmd != "go" {
		t.Errorf("top command base_cmd = %q, want \"go\"", report.TopCommands[0].BaseCmd)
	}
}

func TestQueryStats_HashGrouping(t *testing.T) {
	w := openTestWriter(t)

	// Same command post-redaction, different raw secrets — must produce a single group.
	w.insert(ToolCall{Tool: "shell", StartedAt: time.Now(), Duration: 1 * time.Second, Command: "TOKEN=secret1 git status"})
	w.insert(ToolCall{Tool: "shell", StartedAt: time.Now(), Duration: 2 * time.Second, Command: "TOKEN=secret2 git status"})

	report, err := w.QueryStats(0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.TopCommands) != 1 {
		t.Fatalf("expected 1 command group, got %d: %+v", len(report.TopCommands), report.TopCommands)
	}
	cmd := report.TopCommands[0]
	if cmd.Count != 2 {
		t.Errorf("grouped count = %d, want 2", cmd.Count)
	}
	if cmd.BaseCmd != "git" {
		t.Errorf("base_cmd = %q, want \"git\"", cmd.BaseCmd)
	}
	// AvgMS = (1000 + 2000) / 2 = 1500
	if cmd.AvgMS != 1500 {
		t.Errorf("avg_ms = %.1f, want 1500", cmd.AvgMS)
	}
}

func TestQueryStats_BGHint(t *testing.T) {
	w := openTestWriter(t)

	// StatsP95MinSamples rows of "npm install" at 60 s each.
	for i := 0; i < internal.StatsP95MinSamples; i++ {
		w.insert(ToolCall{
			Tool:      "shell",
			StartedAt: time.Now(),
			Duration:  60 * time.Second,
			Command:   "npm install",
		})
	}

	// threshold = 30 s: p95 (60 s) > threshold → HintBG = true.
	report, err := w.QueryStats(0, 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.TopCommands) == 0 {
		t.Fatal("expected top commands")
	}
	if !report.TopCommands[0].HintBG {
		t.Error("expected HintBG=true when p95 exceeds threshold")
	}

	// threshold = 120 s: p95 (60 s) < threshold → HintBG = false.
	report, err = w.QueryStats(0, 120*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.TopCommands[0].HintBG {
		t.Error("expected HintBG=false when p95 is within threshold")
	}
}

func TestQueryStats_NormalizerVersionFiltering(t *testing.T) {
	w := openTestWriter(t)

	// A row with a stale normalizer version, injected directly.
	staleNV := internal.NormalizerVersion + 1 // definitely not current
	rawInsert(t, w.DB(), "shell", "stalehash", "wget", 1000, staleNV)

	// A row with the current normalizer version.
	w.insert(ToolCall{Tool: "shell", StartedAt: time.Now(), Duration: time.Second, Command: "git fetch"})

	report, err := w.QueryStats(0, time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Tool counts include both rows.
	if len(report.ToolCounts) == 0 || report.ToolCounts[0].Count != 2 {
		t.Errorf("tool counts: expected count=2, got %+v", report.ToolCounts)
	}

	// Top commands only include the current-NV row.
	if len(report.TopCommands) != 1 {
		t.Fatalf("expected 1 top command (current NV only), got %d: %+v",
			len(report.TopCommands), report.TopCommands)
	}
	if report.TopCommands[0].BaseCmd != "git" {
		t.Errorf("top command base_cmd = %q, want \"git\"", report.TopCommands[0].BaseCmd)
	}
}

func TestQueryStats_TopLinesCap(t *testing.T) {
	w := openTestWriter(t)

	// Insert StatsTopLines+5 distinct commands, 1 row each.
	total := internal.StatsTopLines + 5
	for i := 0; i < total; i++ {
		w.insert(ToolCall{
			Tool:      "shell",
			StartedAt: time.Now(),
			Duration:  time.Second,
			Command:   fmt.Sprintf("echo %d", i),
		})
	}

	report, err := w.QueryStats(0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.TopCommands) != internal.StatsTopLines {
		t.Errorf("len(TopCommands) = %d, want %d (StatsTopLines)",
			len(report.TopCommands), internal.StatsTopLines)
	}
}

func TestQueryStats_FileReplaceNotInTopCommands(t *testing.T) {
	w := openTestWriter(t)

	// 5 file_replace rows — no command string, so no cmd_hash.
	for i := 0; i < 5; i++ {
		w.insert(ToolCall{
			Tool:             "file_replace",
			StartedAt:        time.Now(),
			Duration:         100 * time.Millisecond,
			FilePath:         "/projects/foo/bar.go",
			ReplacementCount: 1,
		})
	}

	report, err := w.QueryStats(0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Must appear in tool counts.
	found := false
	for _, ts := range report.ToolCounts {
		if ts.Tool == "file_replace" {
			found = true
			if ts.Count != 5 {
				t.Errorf("file_replace count = %d, want 5", ts.Count)
			}
		}
	}
	if !found {
		t.Error("file_replace missing from tool counts")
	}

	// With no shell/shell_background/setup rows, top commands must be empty.
	if len(report.TopCommands) != 0 {
		t.Errorf("expected 0 top commands, got %d: %+v", len(report.TopCommands), report.TopCommands)
	}
}
