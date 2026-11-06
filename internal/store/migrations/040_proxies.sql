-- 代理库（ADR-022）：把「可复用代理」从主机内联字段提升为独立实体。
--
-- 为什么保留 hosts.proxy_* 四个旧列（038）不删：
--   ① 回滚：删列不可逆，而旧版本仍会按内联字段读代理；
--   ② 一次性代理：用户临时换端口时仍可直接写在主机上，不必先建一条库记录；
--   ③ 删除语义：删除库里的代理时，store 会在同一事务里把引用主机的
--      proxy_id 与内联字段一起归一为「无代理」并清除代理密码（见
--      internal/store/proxies.go 的 DeleteProxy）。旧列保留只为数据库回滚，
--      不再作为删除后的回落来源——否则用户删掉代理后流量仍会从旧副本出去。
-- 解析顺序由会话层固定为 proxy_id → 内联 → 无（internal/session/route.go）。
--
-- 为什么 hosts.proxy_id 是**可空、无外键**：SQLite 的 ALTER TABLE ADD COLUMN
-- 不能追加带 ON DELETE SET NULL 的外键（重建 hosts 表会动到已发布数据，ADR-022
-- 明确禁止）。因此引用完整性由两层保证：
--   ① 应用层：host 保存时校验 proxy_id 存在；删除代理时在同一事务里把引用置空；
--   ② 索引：idx_hosts_proxy 让「按代理统计引用数」与置空更新不必全表扫描。
CREATE TABLE IF NOT EXISTS proxies (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    mode TEXT NOT NULL CHECK (mode IN ('socks5', 'http')),
    host TEXT NOT NULL,
    port INTEGER NOT NULL CHECK (port BETWEEN 1 AND 65535),
    username TEXT NOT NULL DEFAULT '',
    credential_id TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

ALTER TABLE hosts ADD COLUMN proxy_id TEXT;
CREATE INDEX IF NOT EXISTS idx_hosts_proxy ON hosts(proxy_id);

-- 数据迁移：每台仍在用内联代理、且尚未引用库记录的主机各生成一条代理记录，
-- 再回填 hosts.proxy_id。生成规则（ADR-022 §2.3）：
--   - id：proxy_ + 16 字节随机十六进制，与 internal/host 的 randomID 同一形状；
--   - name：主机名优先，空名退回地址，再加「 代理」后缀；
--   - mode/host/port/username/credential_id 原样搬运（凭据仍只是保险库 ID，不复制秘密）。
--
-- 需要一个临时映射表而不是「插完再按字段匹配」：多台主机可能共用完全相同的
-- host/port/username，按字段回填会把它们错配到彼此的代理记录上。映射表按 host_id
-- 一一对应，天然不会串行。临时表不落盘，事务提交后自动消失；IF NOT EXISTS + DELETE
-- 让这段 SQL 在同一连接上重复执行时同样安全（本仓的迁移夹具会重放迁移）。
CREATE TEMP TABLE IF NOT EXISTS proxy_backfill_map (
    host_id TEXT PRIMARY KEY,
    proxy_id TEXT NOT NULL
);
DELETE FROM proxy_backfill_map;

-- 只搬迁「能真正拨号」的内联配置：正常路径上 model.NewHost 已拦下空地址/越界端口，
-- 但老库、人工改库与资产包旁路可能留下脏值。这类行如果硬搬，会撞上 proxies 的
-- CHECK 约束并让整个迁移失败（用户会卡在启动阶段），因此保持它们的内联状态不变——
-- 会话层解析内联字段时仍会给出 session.proxy_invalid，错误归属反而更准确。
INSERT INTO proxy_backfill_map (host_id, proxy_id)
SELECT h.id, 'proxy_' || lower(hex(randomblob(16)))
FROM hosts h
WHERE h.proxy_id IS NULL
  AND h.proxy_mode IN ('socks5', 'http')
  AND TRIM(h.proxy_host) <> ''
  AND h.proxy_port BETWEEN 1 AND 65535;

INSERT INTO proxies (id, name, mode, host, port, username, credential_id, created_at, updated_at)
SELECT m.proxy_id,
       COALESCE(NULLIF(TRIM(h.name), ''), h.address) || ' 代理',
       h.proxy_mode,
       TRIM(h.proxy_host),
       h.proxy_port,
       h.proxy_username,
       h.proxy_credential_id,
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
FROM hosts h
JOIN proxy_backfill_map m ON m.host_id = h.id;

UPDATE hosts
SET proxy_id = (SELECT m.proxy_id FROM proxy_backfill_map m WHERE m.host_id = hosts.id)
WHERE EXISTS (SELECT 1 FROM proxy_backfill_map m WHERE m.host_id = hosts.id);

DROP TABLE proxy_backfill_map;
