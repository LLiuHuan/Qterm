CREATE TABLE snippets (
 id TEXT PRIMARY KEY,
 name TEXT NOT NULL,
 content TEXT NOT NULL,
 group_id TEXT REFERENCES groups(id) ON DELETE SET NULL,
 auto_execute INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE INDEX snippets_group_idx ON snippets(group_id);
