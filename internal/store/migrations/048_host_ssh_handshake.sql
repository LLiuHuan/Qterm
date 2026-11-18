-- 主机级 SSH 握手参数：算法白名单 + ProxyCommand（三列全部 TEXT NOT NULL DEFAULT ''）。
--
-- 为什么是逐主机列，而不是全局设置：
--   算法能力是逐台设备的事实（老交换机只认 group14-sha1，其余几十台都是新算法），
--   全局开关会逼用户每次连老设备前先切一次；更关键的是 HostKeyAlgorithms 属于安全
--   属性（决定信任哪些服务器主机密钥），放宽全局白名单等于给所有主机降级。
--   仓库里同类逐主机字段已有先例：terminal_type、connect_timeout_seconds、x11_*。
--
-- 为什么存 JSON 字符串数组而不是空格分隔：
--   与 027 的 environment_variables、033 的 tags 同一套编解码约定（空串 = 未设置），
--   decodeHostTags / normalizeHostTagsJSON 的姿态可以照搬：格式坏了明确报错，
--   而不是静默当作"没有设置"让用户以为配置丢了。
--
-- 为什么空串表示"用内置默认"：
--   旧库升级后三列全空，连接行为与升级前逐字节相同；只有用户真正填了才覆盖。
--   这样迁移本身零风险，不需要回填任何数据。
--
-- ssh_proxy_command 存的是原始命令串（含 %h/%p 占位符，不展开）：
--   展开发生在每次连接时、按当次目标的地址端口替换，因此同一条命令可以被多台
--   主机复用。命令可能含凭据（如 --parameters token=...），读路径不得写日志。
ALTER TABLE hosts ADD COLUMN ssh_host_key_algorithms TEXT NOT NULL DEFAULT '';
ALTER TABLE hosts ADD COLUMN ssh_key_exchanges TEXT NOT NULL DEFAULT '';
ALTER TABLE hosts ADD COLUMN ssh_proxy_command TEXT NOT NULL DEFAULT '';
