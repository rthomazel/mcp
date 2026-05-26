BEGIN;

CREATE TABLE tool_calls (
    id                    TEXT     PRIMARY KEY,
    tool                  TEXT     NOT NULL,
    called_at             DATETIME NOT NULL,
    duration_ms           INTEGER  NOT NULL DEFAULT 0,
    server_version        TEXT,
    error_kind            TEXT,

    base_cmd              TEXT,
    cmd_hash              TEXT,
    cmd_encrypted         TEXT,
    normalizer_version    INTEGER,
    exit_code             INTEGER,
    timed_out             INTEGER  NOT NULL DEFAULT 0 CHECK (timed_out IN (0, 1)),
    cwd                   TEXT,
    job_id                TEXT,
    redacted_byte_counts  TEXT,

    file_path             TEXT,
    replacement_count     INTEGER,
    replacement_bytes     TEXT,
    dry_run               INTEGER CHECK (dry_run IS NULL OR dry_run IN (0, 1)),

    setup_paths           TEXT
);

CREATE INDEX idx_tc_called_at      ON tool_calls (called_at);
CREATE INDEX idx_tc_tool_date      ON tool_calls (tool, called_at);
CREATE INDEX idx_tc_tool_hash_date ON tool_calls (tool, cmd_hash, called_at);
CREATE INDEX idx_tc_file_path      ON tool_calls (tool, file_path);

COMMIT;
