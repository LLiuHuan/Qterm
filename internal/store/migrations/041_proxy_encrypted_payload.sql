-- 代理记录的直接密码只允许进入统一加密 payload（ADR-022 修订）。
--
-- 直接密码不新增明文列，也不写回 ProxyRecord。AAD 域固定为 proxy，记录 ID 绑定
-- 密文归属；代理库不再关联凭据，credential_id 由后续迁移删除。
ALTER TABLE proxies ADD COLUMN encrypted_payload BLOB;
