-- SSH 主机的可选双因素认证策略。
--
-- 主机只保存全局 OTP 条目 ID，不复制 OTP 密钥；旧主机默认关闭，且不影响既有连接方式。
ALTER TABLE hosts ADD COLUMN ssh_two_factor_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE hosts ADD COLUMN ssh_otp_entry_id TEXT NOT NULL DEFAULT '';
ALTER TABLE hosts ADD COLUMN ssh_otp_auto_fill INTEGER NOT NULL DEFAULT 0;
