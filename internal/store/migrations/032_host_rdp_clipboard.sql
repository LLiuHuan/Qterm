-- 每主机的远程桌面剪贴板方向（对应 FreeRDP 的 ClipboardFeatureMask）：
--   off             = 关闭：不挂 CLIPRDR 通道，两侧都拿不到对方剪贴板
--   both            = 双向（默认，与改动前的行为一致）
--   local-to-remote = 只允许本机 → 远程
--   remote-to-local = 只允许远程 → 本机
ALTER TABLE hosts ADD COLUMN rdp_clipboard_mode TEXT NOT NULL DEFAULT 'both'
    CHECK (rdp_clipboard_mode IN ('off', 'both', 'local-to-remote', 'remote-to-local'));

-- 允许通过剪贴板交换的内容：
--   text           = 仅文本（默认，也是当前唯一真正可用的取值）
--   text-and-files = 文本与文件（**契约预留**：本机侧的文件剪贴板读写尚未实现，
--                    界面暂不放开该选项，下发前也会被收敛成文本，见 internal/app/rdp.go）
ALTER TABLE hosts ADD COLUMN rdp_clipboard_content TEXT NOT NULL DEFAULT 'text'
    CHECK (rdp_clipboard_content IN ('text', 'text-and-files'));
