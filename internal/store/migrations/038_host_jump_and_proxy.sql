-- 跳板机与代理的第一个数据维度（ADR-020 / P1+P2 实施契约 §1.1）。
--
-- 跳板建模成「引用另一台已保存的 SSH 主机」而不是新实体表：跳板机的地址/端口/用户名/
-- 凭据/指纹/图标/备注全部复用既有主机列，多跳靠每台主机各自的 jump_host_id 串起来
-- （深度上限 5 由服务层校验，本列只负责关系与引用完整性）。
-- ON DELETE RESTRICT 是删除保护的兜底：服务层会先查引用计数并返回专用错误键，
-- 但「并发删除」或将来新增的旁路写入口不能绕过数据库这一层。
--
-- proxy_credential_id **刻意留在明文列**（不进 encrypted_hosts.go 的加密 payload）：
-- 它只是一个指向保险库凭据的 ID，本身不含秘密（ADR-004：秘密只以凭据 ID 存在）；
-- 而把它塞进 payload 需要 payload schema 版本迁移，代价与收益不匹配。
-- 代理密码因此**只能**通过该凭据 ID 提供，不新增 proxy_password_source：
-- 避免出现「主机自带明文代理密码」这第二条秘密路径。
--
-- proxy_mode 的 CHECK 与 model 层的白名单同口径：写进一个实现不认识的取值只会变成
-- 「看起来配了代理、实际行为不明」。默认 'none' + 其余三项默认空/0 ⇒ 老库与老资产包
-- 升级后行为逐字节不变。
ALTER TABLE hosts ADD COLUMN jump_host_id TEXT REFERENCES hosts(id) ON DELETE RESTRICT;
ALTER TABLE hosts ADD COLUMN proxy_mode TEXT NOT NULL DEFAULT 'none' CHECK (proxy_mode IN ('none','socks5','http'));
ALTER TABLE hosts ADD COLUMN proxy_host TEXT NOT NULL DEFAULT '';
ALTER TABLE hosts ADD COLUMN proxy_port INTEGER NOT NULL DEFAULT 0;
ALTER TABLE hosts ADD COLUMN proxy_username TEXT NOT NULL DEFAULT '';
ALTER TABLE hosts ADD COLUMN proxy_credential_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_hosts_jump_host ON hosts(jump_host_id);
