package stats

import (
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"

	"github.com/rthomazel/bench-mcp/internal"
)

// StatsReport is the result of QueryStats.
type StatsReport struct {
	Window      string
	ToolCounts  []ToolStat
	TopCommands []CmdStat
	HasKey      bool
}

// ToolStat holds aggregate stats for one tool.
type ToolStat struct {
	Tool  string
	Count int64
	AvgMS float64
	P95MS *int64 // nil when sample size < 20
}

// CmdStat holds aggregate stats for one (normalizer_version, cmd_hash) group.
type CmdStat struct {
	BaseCmd    string
	HashPrefix string
	Count      int64
	AvgMS      float64
	P95MS      *int64
	Command    string // decrypted redacted command; empty when no key
	HintBG     bool   // p95 > bgHintThreshold
}

func buildDateFilter(days int) (filter, window string) {
	if days <= 0 {
		return "", "all time"
	}
	return fmt.Sprintf("AND called_at > datetime('now', '-%d days')", days),
		fmt.Sprintf("last %d days", days)
}

func queryToolCounts(conn *sql.DB, filter string) ([]ToolStat, error) {
	query := `
		SELECT tool, COUNT(*) AS cnt, AVG(duration_ms) AS avg_ms
		FROM tool_calls
		WHERE tool IS NOT NULL ` + filter + `
		GROUP BY tool
		ORDER BY cnt DESC`

	rows, err := conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query tool counts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	// Collect all duration_ms per tool for p95 computation.
	type toolRow struct {
		tool  string
		count int64
		avgMS float64
	}
	var rows2 []toolRow
	for rows.Next() {
		var r toolRow
		if err := rows.Scan(&r.tool, &r.count, &r.avgMS); err != nil {
			return nil, fmt.Errorf("scan tool row: %w", err)
		}
		rows2 = append(rows2, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Fetch durations per tool for p95.
	stats := make([]ToolStat, 0, len(rows2))
	for _, r := range rows2 {
		durations, err := fetchDurations(conn, filter, "tool = ?", r.tool)
		if err != nil {
			slog.Warn("stats: fetch tool durations", "tool", r.tool, "err", err)
		}
		stat := ToolStat{
			Tool:  r.tool,
			Count: r.count,
			AvgMS: r.avgMS,
			P95MS: p95(durations),
		}
		stats = append(stats, stat)
	}
	return stats, nil
}

func queryTopCommands(conn *sql.DB, filter string, bgHintThreshold time.Duration, encKey []byte) ([]CmdStat, error) {
	currentNV := internal.NormalizerVersion
	query := fmt.Sprintf(`
		SELECT base_cmd, cmd_hash, normalizer_version, duration_ms, cmd_encrypted
		FROM tool_calls
		WHERE tool IN ('shell', 'shell_background', 'setup')
		  AND cmd_hash IS NOT NULL
		  AND normalizer_version = %d
		  %s
		ORDER BY cmd_hash`, currentNV, filter)

	rows, err := conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query top commands: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type group struct {
		baseCmd   string
		hash      string
		durations []int64
		encrypted string // last non-empty encrypted value
	}
	groups := make(map[string]*group)
	var order []string

	for rows.Next() {
		var baseCmd, cmdHash sql.NullString
		var normVer sql.NullInt64
		var durationMS int64
		var cmdEncrypted sql.NullString

		if err := rows.Scan(&baseCmd, &cmdHash, &normVer, &durationMS, &cmdEncrypted); err != nil {
			return nil, fmt.Errorf("scan cmd row: %w", err)
		}

		hash := cmdHash.String
		if _, ok := groups[hash]; !ok {
			groups[hash] = &group{baseCmd: baseCmd.String, hash: hash}
			order = append(order, hash)
		}
		g := groups[hash]
		g.durations = append(g.durations, durationMS)
		if cmdEncrypted.String != "" {
			g.encrypted = cmdEncrypted.String
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Sort groups by count descending.
	sort.Slice(order, func(i, j int) bool {
		return len(groups[order[i]].durations) > len(groups[order[j]].durations)
	})

	const maxTopCommands = 20
	thresholdMS := bgHintThreshold.Milliseconds()

	cmdStats := make([]CmdStat, 0, len(order))
	for _, hash := range order {
		if len(cmdStats) >= maxTopCommands {
			break
		}
		g := groups[hash]
		p := p95(g.durations)

		var decrypted string
		if len(encKey) > 0 && g.encrypted != "" {
			plain, err := Decrypt(encKey, g.encrypted)
			if err == nil {
				decrypted = plain
			}
		}

		hashPrefix := hash
		if len(hash) > 6 {
			hashPrefix = hash[:6]
		}

		hintBG := false
		if p != nil && *p > thresholdMS {
			hintBG = true
		}

		var avgMS float64
		for _, d := range g.durations {
			avgMS += float64(d)
		}
		if len(g.durations) > 0 {
			avgMS /= float64(len(g.durations))
		}

		cmdStats = append(cmdStats, CmdStat{
			BaseCmd:    g.baseCmd,
			HashPrefix: hashPrefix,
			Count:      int64(len(g.durations)),
			AvgMS:      avgMS,
			P95MS:      p,
			Command:    decrypted,
			HintBG:     hintBG,
		})
	}
	return cmdStats, nil
}

func fetchDurations(conn *sql.DB, filter, condition, arg string) ([]int64, error) {
	query := `SELECT duration_ms FROM tool_calls WHERE ` + condition + ` ` + filter
	rows, err := conn.Query(query, arg)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var durations []int64
	for rows.Next() {
		var d int64
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		durations = append(durations, d)
	}
	return durations, rows.Err()
}

// p95 returns the 95th percentile using the nearest-rank formula.
// Returns nil when the sample size is below 20.
func p95(durations []int64) *int64 {
	const minSamples = 20
	if len(durations) < minSamples {
		return nil
	}
	sorted := make([]int64, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	// nearest-rank: index = ceil(0.95 * n) - 1
	idx := int(math.Ceil(0.95*float64(len(sorted)))) - 1
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	v := sorted[idx]
	return &v
}
