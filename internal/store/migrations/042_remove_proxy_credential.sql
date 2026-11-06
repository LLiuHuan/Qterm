-- 代理库只支持自身的加密直接密码，不再关联凭据保险库。
--
-- 040 是已发布迁移，保留其历史定义；这里删除旧结构化引用及其中的值。
ALTER TABLE proxies DROP COLUMN credential_id;
