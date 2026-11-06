CREATE TABLE terminal_recordings (
 id TEXT PRIMARY KEY,
 session_id TEXT NOT NULL,
 host_id TEXT NOT NULL,
 path TEXT NOT NULL,
 started_at TEXT NOT NULL,
 ended_at TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL,
 bytes INTEGER NOT NULL DEFAULT 0,
 event_count INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX terminal_recordings_started_idx ON terminal_recordings(started_at DESC);
CREATE INDEX terminal_recordings_session_idx ON terminal_recordings(session_id);
