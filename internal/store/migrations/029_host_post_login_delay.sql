-- 每主机「登录后执行命令」的延迟秒数（0 = 立即执行）；登录后命令本身支持多行。
ALTER TABLE hosts ADD COLUMN post_login_command_delay_seconds INTEGER NOT NULL DEFAULT 0 CHECK (post_login_command_delay_seconds BETWEEN 0 AND 300);
