-- 每主机的终端与连接行为：退格键字节、终端编码、连接超时、登录后命令、登录后环境变量。
-- environment_variables 存 JSON 数组（[{name,value}]）；值可能含令牌，与其他主机字段同级保存。
ALTER TABLE hosts ADD COLUMN backspace_mode TEXT NOT NULL DEFAULT 'del' CHECK (backspace_mode IN ('del', 'bs'));
ALTER TABLE hosts ADD COLUMN encoding TEXT NOT NULL DEFAULT 'utf-8' CHECK (encoding IN ('utf-8', 'gbk'));
ALTER TABLE hosts ADD COLUMN connect_timeout_seconds INTEGER NOT NULL DEFAULT 15 CHECK (connect_timeout_seconds BETWEEN 1 AND 300);
ALTER TABLE hosts ADD COLUMN post_login_command TEXT NOT NULL DEFAULT '';
ALTER TABLE hosts ADD COLUMN environment_variables TEXT NOT NULL DEFAULT '';
