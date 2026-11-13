-- 清理 045/046 引入的主机别名（alias）与自定义 ID（custom_id）。
--
-- 为什么"删功能"必须写成一条新的前进迁移，而不是把 045/046 两个文件删掉：
--   045/046 已经随 v0.0.4 / v0.0.5 发布出去，装过这两版的用户库 user_version 就是 46。
--   迁移版本只能前进：文件被删、可用版本回落到 44 后，这些库会以
--   "schema version 46 exceeds supported version 44" 拒绝启动，而且没有回归路径。
--   所以 045/046 原样留在迁移链上，"移除功能"用本条新迁移表达。
--
-- 为什么先删索引再删列：
--   SQLite 的 ALTER TABLE ... DROP COLUMN 不允许删除被索引引用的列，
--   hosts_custom_id_idx 是 046 建在 custom_id 上的部分唯一索引，必须先撤掉。
--
-- 数据影响（刻意为之）：
--   已经升到 46 的库里，alias / custom_id 的取值会在本迁移后丢弃。两个功能已从
--   产品移除，列不会再被任何读写路径引用，留着只会是无人维护的死字段；
--   用户要找回这些值只能从导出的资产包重建，本迁移不做数据搬运。
DROP INDEX IF EXISTS hosts_custom_id_idx;
ALTER TABLE hosts DROP COLUMN custom_id;
ALTER TABLE hosts DROP COLUMN alias;
