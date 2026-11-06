ALTER TABLE secrets ADD COLUMN name TEXT NOT NULL DEFAULT '';

-- 旧版本没有展示名称；使用稳定 ID 回填，避免迁移后生成不可管理的空记录。
UPDATE secrets SET name = id WHERE name = '';
