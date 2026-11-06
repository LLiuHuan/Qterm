-- 每主机的远程桌面显示方式：
--   fit   = 自适应窗口（连接/窗口尺寸变化时按窗口大小协商，超出部分自动缩放）
--   fixed = 固定分辨率（始终按 rdp_width × rdp_height 连接，画面等比缩放填充）
ALTER TABLE hosts ADD COLUMN rdp_display_mode TEXT NOT NULL DEFAULT 'fit'
    CHECK (rdp_display_mode IN ('fit', 'fixed'));
ALTER TABLE hosts ADD COLUMN rdp_width INTEGER NOT NULL DEFAULT 1280
    CHECK (rdp_width BETWEEN 640 AND 7680);
ALTER TABLE hosts ADD COLUMN rdp_height INTEGER NOT NULL DEFAULT 800
    CHECK (rdp_height BETWEEN 480 AND 4320);
