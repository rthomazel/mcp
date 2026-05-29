package stats

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	_ "modernc.org/sqlite" // register "sqlite" driver

	"github.com/rthomazel/bench-mcp/db"
	"github.com/rthomazel/bench-mcp/internal"
)

const writeQueueCap = 256

// ToolCall holds the data for one tool invocation to be recorded.
type ToolCall struct {
	Tool      string
	StartedAt time.Time
	Duration  time.Duration
	ErrorKind string // "" = success; "timeout", "start_failed", "arg_error", "write_error"

	// shell / shell_background / setup
	Command  string
	ExitCode *int
	TimedOut bool
	CWD      string
	JobID    string

	// file_replace / file_replace_all
	FilePath         string
	ReplacementCount int
	ReplacementBytes [][2]int // [[find_bytes, replace_bytes], ...]
	DryRun           *bool

	// setup
	SetupPaths []string
}

// WriterConfig holds static configuration for a Writer.
type WriterConfig struct {
	ServerVersion  string
	EncryptionKey  []byte
	RedactPatterns []*regexp.Regexp
}

// Writer records ToolCall rows to SQLite asynchronously.
type Writer struct {
	db  *sql.DB
	ch  chan ToolCall
	cfg WriterConfig
	wg  sync.WaitGroup
}

// Open opens (or creates) the stats database at dbPath, runs migrations, and
// starts the background write goroutine.
func Open(dbPath string, cfg WriterConfig) (*Writer, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)", dbPath)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open stats db: %w", err)
	}
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(0)
	if _, err = conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("set WAL: %w", err)
	}
	if err = internal.MigrateDB(conn, db.Migrations); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("migrate stats db: %w", err)
	}
	w := &Writer{
		db:  conn,
		ch:  make(chan ToolCall, writeQueueCap),
		cfg: cfg,
	}
	w.wg.Add(1)
	go w.drain()
	return w, nil
}

// Close drains the write queue (up to 1 minute) then closes the database.
func (w *Writer) Close() {
	close(w.ch)
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Minute):
		slog.Error("stats: shutdown timed out, some pending records may be lost")
	}
	_ = w.db.Close()
}

// Record enqueues tc for async insertion. Drops and logs if the queue is full.
func (w *Writer) Record(tc ToolCall) {
	select {
	case w.ch <- tc:
	default:
		slog.Warn("stats: write queue full, record dropped", "tool", tc.Tool)
	}
}

// DB returns the underlying *sql.DB for read queries.
func (w *Writer) DB() *sql.DB { return w.db }

func (w *Writer) drain() {
	defer w.wg.Done()
	for tc := range w.ch {
		w.insert(tc)
	}
}

func (w *Writer) insert(tc ToolCall) {
	var (
		baseCmd, cmdHash, cmdEncrypted string
		normalizerVer                  *int
		redactedByteCountsJSON         *string
	)

	if tc.Command != "" {
		processed := ProcessCommand(tc.Command, w.cfg.RedactPatterns)
		baseCmd = processed.BaseCmd
		cmdHash = processed.Hash

		if len(w.cfg.EncryptionKey) > 0 {
			enc, err := Encrypt(w.cfg.EncryptionKey, processed.Normalized)
			if err != nil {
				slog.Error("stats: encrypt failed", "err", err)
			} else {
				cmdEncrypted = enc
			}
		}

		nv := internal.NormalizerVersion
		normalizerVer = &nv

		if len(processed.RedactedByteCounts) > 0 {
			js, err := json.Marshal(processed.RedactedByteCounts)
			if err == nil {
				s := string(js)
				redactedByteCountsJSON = &s
			}
		}
	}

	var replacementBytesJSON *string
	if len(tc.ReplacementBytes) > 0 {
		js, err := json.Marshal(tc.ReplacementBytes)
		if err == nil {
			s := string(js)
			replacementBytesJSON = &s
		}
	}

	var setupPathsJSON *string
	if len(tc.SetupPaths) > 0 {
		js, err := json.Marshal(tc.SetupPaths)
		if err == nil {
			s := string(js)
			setupPathsJSON = &s
		}
	}

	var dryRunInt *int
	if tc.DryRun != nil {
		v := 0
		if *tc.DryRun {
			v = 1
		}
		dryRunInt = &v
	}

	timedOut := 0
	if tc.TimedOut {
		timedOut = 1
	}

	calledAt := tc.StartedAt.UTC().Format("2006-01-02 15:04:05")
	durationMS := tc.Duration.Milliseconds()

	_, err := w.db.Exec(`
		INSERT INTO tool_calls (
			id, tool, called_at, duration_ms, server_version, error_kind,
			base_cmd, cmd_hash, cmd_encrypted, normalizer_version, exit_code, timed_out, cwd, job_id, redacted_byte_counts,
			file_path, replacement_count, replacement_bytes, dry_run,
			setup_paths
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(),
		tc.Tool,
		calledAt,
		durationMS,
		nullString(w.cfg.ServerVersion),
		nullString(tc.ErrorKind),
		nullString(baseCmd),
		nullString(cmdHash),
		nullString(cmdEncrypted),
		normalizerVer,
		tc.ExitCode,
		timedOut,
		nullString(tc.CWD),
		nullString(tc.JobID),
		redactedByteCountsJSON,
		nullString(tc.FilePath),
		nullInt(tc.ReplacementCount),
		replacementBytesJSON,
		dryRunInt,
		setupPathsJSON,
	)
	if err != nil {
		slog.Error("stats: insert failed", "tool", tc.Tool, "err", err)
	}
}

func nullString(s string) any { return lo.Ternary[any](s == "", nil, s) }
func nullInt(n int) any       { return lo.Ternary[any](n == 0, nil, n) }

// QueryStats queries the stats DB for a summary over the given rolling window.
// days=0 returns all time. bgHintThreshold is half the shell timeout for the hint.
func (w *Writer) QueryStats(days int, bgHintThreshold time.Duration) (*StatsReport, error) {
	filter, window := buildDateFilter(days)

	toolCounts, err := queryToolCounts(w.db, filter)
	if err != nil {
		return nil, err
	}

	topCmds, err := queryTopCommands(w.db, filter, bgHintThreshold, w.cfg.EncryptionKey)
	if err != nil {
		return nil, err
	}

	return &StatsReport{
		Window:      window,
		ToolCounts:  toolCounts,
		TopCommands: topCmds,
		HasKey:      len(w.cfg.EncryptionKey) > 0,
	}, nil
}
