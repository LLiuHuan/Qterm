-- 命令历史：记录用户在 Qterm 中提交过的命令。
-- 命令文本可能包含敏感参数，因此只保存在本机数据库，不进入日志、错误详情或事件载荷。
CREATE TABLE command_history (
    id TEXT PRIMARY KEY,
    host_id TEXT NOT NULL,
    host_name TEXT NOT NULL,
    session_id TEXT NOT NULL,
    command TEXT NOT NULL,
    executed_at TEXT NOT NULL
);
CREATE INDEX command_history_host_idx ON command_history(host_id, executed_at DESC);
