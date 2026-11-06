-- 命令分类独立于资产分组，保留已有分类并将旧关联转换为主机快照。
CREATE TABLE snippet_groups (
 id TEXT PRIMARY KEY,
 name TEXT NOT NULL COLLATE NOCASE UNIQUE,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
INSERT INTO snippet_groups SELECT DISTINCT g.id,g.name,g.created_at,g.updated_at
 FROM groups g JOIN snippets s ON s.group_id=g.id;
ALTER TABLE snippets ADD COLUMN snippet_group_id TEXT REFERENCES snippet_groups(id) ON DELETE RESTRICT;
ALTER TABLE snippets ADD COLUMN asset_group_id TEXT REFERENCES groups(id) ON DELETE SET NULL;
UPDATE snippets SET snippet_group_id=group_id;
UPDATE snippets SET asset_group_id=group_id;
UPDATE snippets SET host_ids=(
 SELECT json_group_array(id) FROM (
 SELECT value AS id FROM json_each(COALESCE(snippets.host_ids,'[]'))
 UNION SELECT id FROM hosts WHERE group_id=snippets.group_id
 )) WHERE group_id IS NOT NULL;
UPDATE snippets SET group_id=NULL;
