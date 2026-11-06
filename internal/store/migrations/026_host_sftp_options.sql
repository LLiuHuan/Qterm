-- 每主机的 SFTP 偏好覆盖：展示隐藏文件（inherit/enabled/disabled，与目录跟随同构）与默认打开目录（远端绝对路径，空 = 不指定）。
ALTER TABLE hosts ADD COLUMN sftp_show_hidden_files TEXT NOT NULL DEFAULT 'inherit' CHECK (sftp_show_hidden_files IN ('inherit', 'enabled', 'disabled'));
ALTER TABLE hosts ADD COLUMN sftp_default_directory TEXT NOT NULL DEFAULT '';
