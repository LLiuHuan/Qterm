CREATE TABLE session_history (
    id TEXT PRIMARY KEY,
    host_id TEXT NOT NULL,
    host_name TEXT NOT NULL,
    address TEXT NOT NULL,
    started_at TEXT NOT NULL,
    ended_at TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    error_key TEXT NOT NULL DEFAULT ''
);
CREATE INDEX session_history_started_idx ON session_history(started_at DESC);
