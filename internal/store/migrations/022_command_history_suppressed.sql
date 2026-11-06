-- 联想补全隐藏标识：保留历史记录，但从补全建议中过滤；重新执行会新增未隐藏记录。
ALTER TABLE command_history ADD COLUMN suppressed INTEGER NOT NULL DEFAULT 0;
