-- 主机自定义 ID。
--
-- 与主键 id 的区别：id 是内部随机标识（永不变、不对外承诺稳定），custom_id 是
-- 用户自己起的**稳定别名**，用于 CLI、脚本与将来的 URL scheme 引用主机。
-- 因此它必须唯一（大小写不敏感），且可以为空（不是每台主机都需要）。
--
-- 用独立列而不是替换 id：id 被 hosts_jump、proxies、会话、录制等处的引用依赖，
-- 改主键语义的代价远大于加一列。
ALTER TABLE hosts ADD COLUMN custom_id TEXT NOT NULL DEFAULT '';

-- 部分索引：只约束非空值。空串表示"没设置"，多台主机都可以为空。
-- COLLATE NOCASE 让 DB1 与 db1 视为同一个（模型层还会统一转小写）。
CREATE UNIQUE INDEX hosts_custom_id_idx ON hosts(custom_id COLLATE NOCASE) WHERE custom_id <> '';
