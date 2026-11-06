-- 每主机的连接类型与终端类型：standard/network，以及上报给远端的 TERM。
ALTER TABLE hosts ADD COLUMN connection_type TEXT NOT NULL DEFAULT 'standard' CHECK (connection_type IN ('standard', 'network'));
ALTER TABLE hosts ADD COLUMN terminal_type TEXT NOT NULL DEFAULT 'xterm-256color';
