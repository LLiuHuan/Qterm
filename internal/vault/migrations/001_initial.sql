CREATE TABLE secrets (
    id TEXT PRIMARY KEY,
    kind INTEGER NOT NULL CHECK (kind BETWEEN 1 AND 3),
    envelope BLOB NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX secrets_kind_idx ON secrets(kind);
