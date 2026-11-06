-- migrations.go 在同一事务中兼容两种 v10 的列结构；已保存主机快照不变。
CREATE INDEX IF NOT EXISTS snippets_asset_group_idx ON snippets(asset_group_id);
