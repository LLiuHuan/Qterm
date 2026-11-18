-- 扩展凭据类型以容纳 OpenSSH 用户证书（kind = 5）。
--
-- 为什么要重建表：旧表的 CHECK 约束写死了 1..4，SQLite 无法修改约束，
-- 只能按「建新表 → 拷数据 → 换名」的路径升级。这与 004 的做法一致。
--
-- 与 004 的区别：这次要**逐列**把已有数据搬过来。004 漏掉了 username 与
-- public_key，登录信息因此丢过一次；迁移里少搬一列的表现是「升级后凭据还在、
-- 但用户名/公钥变空」，比直接报错更难发现。
CREATE TABLE secrets_v5 (
    id TEXT PRIMARY KEY,
    kind INTEGER NOT NULL CHECK (kind BETWEEN 1 AND 5),
    envelope BLOB NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    username TEXT NOT NULL DEFAULT '',
    public_key TEXT NOT NULL DEFAULT ''
);

INSERT INTO secrets_v5(id, kind, envelope, created_at, updated_at, name, username, public_key)
SELECT id, kind, envelope, created_at, updated_at, name, username, public_key FROM secrets;

DROP TABLE secrets;
ALTER TABLE secrets_v5 RENAME TO secrets;
CREATE INDEX secrets_kind_idx ON secrets(kind);
