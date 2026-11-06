-- 每主机的 VNC 选项（ADR-017 路线 C：RFB 由嵌入的 noVNC 实现，Go 侧只做字节桥）。
--
-- view_only：只读模式。开启后前端不发送任何输入事件（由 noVNC 的 viewOnly 保证）；
--            Go 侧的字节桥无法区分"输入字节"与"协议请求字节"，因此不做字节级过滤。
ALTER TABLE hosts ADD COLUMN vnc_view_only INTEGER NOT NULL DEFAULT 0
    CHECK (vnc_view_only IN (0, 1));

-- shared：对应 RFB ClientInit 的 shared 标志。
--   1 = 共享（默认，与主流客户端一致）：允许其他客户端保持连接；
--   0 = 独占：服务端会断开其他已连接的客户端。
ALTER TABLE hosts ADD COLUMN vnc_shared INTEGER NOT NULL DEFAULT 1
    CHECK (vnc_shared IN (0, 1));

-- pixel_depth：请求的色深。32 = depth 24 真彩（默认）；16 = 弱网降色深。
-- 8bpp + colour map 不在 v1 范围（需要额外的色表协议，收益低）。
ALTER TABLE hosts ADD COLUMN vnc_pixel_depth INTEGER NOT NULL DEFAULT 32
    CHECK (vnc_pixel_depth IN (16, 32));
