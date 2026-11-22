-- Outbox of events. See spc/notifications/spec.md.
--
-- A row stays pending until processed_at or given_up_at is set. handled_by
-- names the handlers that already succeeded, so a retry skips them.
CREATE TABLE events (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    type          TEXT        NOT NULL,
    subject       TEXT,
    data          JSONB       NOT NULL DEFAULT '{}',
    failures      INTEGER     NOT NULL DEFAULT 0,
    backoff_until TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expired_at    TIMESTAMPTZ NOT NULL,
    processed_at  TIMESTAMPTZ,
    given_up_at   TIMESTAMPTZ,
    handled_by    JSONB       NOT NULL DEFAULT '[]',

    -- Mirrors domain.ValidEventType and DraftEvent.Validate on purpose, to
    -- backstop manual inserts.
    CONSTRAINT events_type CHECK (type ~ '^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$'),
    CONSTRAINT events_subject CHECK (subject IS NULL OR btrim(subject) <> ''),
    CONSTRAINT events_data CHECK (jsonb_typeof(data) = 'object'),
    CONSTRAINT events_failures CHECK (failures >= 0)
);

-- Backs ListDueEvents: only events still in play.
CREATE INDEX events_pending ON events (created_at, id)
    WHERE processed_at IS NULL AND given_up_at IS NULL;

-- Makes RecordEvent atomic: one event per type and subject.
CREATE UNIQUE INDEX events_type_subject ON events (type, subject)
    WHERE subject IS NOT NULL;
