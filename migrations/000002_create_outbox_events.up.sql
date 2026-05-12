CREATE TABLE IF NOT EXISTS outbox_events
(
    id           uuid PRIMARY KEY,
    event_type   text        NOT NULL,
    aggregate_id uuid        NOT NULL,
    payload      jsonb       NOT NULL,
    attempts     integer     NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error   text,
    available_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_pending
    ON outbox_events (available_at, created_at)
    WHERE published_at IS NULL;
