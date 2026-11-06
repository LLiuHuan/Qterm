-- 未启用 payload 加密的 Store 也必须完整保存高级连接设置。
ALTER TABLE hosts ADD COLUMN ssh_keep_alive INTEGER NOT NULL DEFAULT 1 CHECK (ssh_keep_alive IN (0, 1));
ALTER TABLE hosts ADD COLUMN latency_detection INTEGER NOT NULL DEFAULT 1 CHECK (latency_detection IN (0, 1));
ALTER TABLE hosts ADD COLUMN auto_detect_os_icon INTEGER NOT NULL DEFAULT 1 CHECK (auto_detect_os_icon IN (0, 1));
ALTER TABLE hosts ADD COLUMN directory_follow TEXT NOT NULL DEFAULT 'inherit' CHECK (directory_follow IN ('inherit', 'enabled', 'disabled'));
ALTER TABLE hosts ADD COLUMN password_source TEXT NOT NULL DEFAULT 'none' CHECK (password_source IN ('none', 'prompt', 'saved'));
