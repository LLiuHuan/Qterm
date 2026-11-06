-- 扩展凭据类型并保存非秘密的连接元数据。旧表的 CHECK 约束只允许 1..3，
-- 因此重建表而不是仅增加列，避免旧数据库拒绝 SSH Agent 记录。
CREATE TABLE secrets_v4 (
    id TEXT PRIMARY KEY,
    kind INTEGER NOT NULL CHECK (kind BETWEEN 1 AND 4),
    envelope BLOB NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    username TEXT NOT NULL DEFAULT '',
    public_key TEXT NOT NULL DEFAULT ''
);

INSERT INTO secrets_v4(id, kind, envelope, created_at, updated_at, name)
SELECT id, kind, envelope, created_at, updated_at, name FROM secrets;

DROP TABLE secrets;
ALTER TABLE secrets_v4 RENAME TO secrets;
CREATE INDEX secrets_kind_idx ON secrets(kind);
