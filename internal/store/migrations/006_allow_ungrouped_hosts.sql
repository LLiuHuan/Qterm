-- 允许主机位于根级“未分组”区域，NULL 表示没有所属分组。
CREATE TABLE hosts_v2 (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    address TEXT NOT NULL,
    port INTEGER NOT NULL CHECK (port BETWEEN 1 AND 65535),
    username TEXT NOT NULL,
    group_id TEXT REFERENCES groups(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    credential_id TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    protocol TEXT NOT NULL DEFAULT 'ssh',
    icon TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

INSERT INTO hosts_v2 (
    id, name, address, port, username, group_id, credential_id, notes, protocol, icon, created_at, updated_at
)
SELECT id, name, address, port, username, group_id, credential_id, notes, protocol, icon, created_at, updated_at
FROM hosts;

DROP TABLE hosts;
ALTER TABLE hosts_v2 RENAME TO hosts;

CREATE INDEX hosts_group_id_idx ON hosts(group_id);
CREATE INDEX hosts_name_idx ON hosts(name COLLATE NOCASE);
CREATE INDEX hosts_credential_id_idx ON hosts(credential_id);
CREATE INDEX hosts_protocol_idx ON hosts(protocol);
