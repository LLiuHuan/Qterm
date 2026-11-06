-- 未归档历史受保留上限约束；归档历史由用户明确保留，不能被自动裁剪。
ALTER TABLE session_history ADD COLUMN archived INTEGER NOT NULL DEFAULT 0;
CREATE INDEX session_history_archived_started_idx ON session_history(archived, started_at DESC);
