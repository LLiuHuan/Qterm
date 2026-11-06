-- 自定义内联代理也只支持自身加密直接密码，不再关联凭据保险库。
--
-- 038 是已发布迁移，保留其历史定义；这里删除旧结构化引用及其中的值。
ALTER TABLE hosts DROP COLUMN proxy_credential_id;
