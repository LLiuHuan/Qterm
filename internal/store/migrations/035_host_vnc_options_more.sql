-- 每主机 VNC 面板的显示与重连选项（承接 034 的三项协议级开关）。
--
-- scale：画面缩放方式。fit = 适应窗口（默认，与 RDP 的 rdp_display_mode 同名同义）；
--        actual = 实际大小（1:1，超出部分靠滚动查看，适合做像素级操作）。
--        它只影响前端 noVNC 的视口渲染（scaleViewport/clipViewport），
--        不参与 RFB 握手，因此列里存的是"界面偏好"而不是安全开关。
ALTER TABLE hosts ADD COLUMN vnc_scale TEXT NOT NULL DEFAULT 'fit'
    CHECK (vnc_scale IN ('fit', 'actual'));

-- auto_reconnect：会话意外断开后是否自动重连。0 = 关闭（默认，与多数终端行为一致，
--   断线原因对用户可见）；1 = 开启。
--   风险：自动重连会重新走一次 RFB 认证。凭据来源是"每次连接询问"（password_source=prompt）
--   时无法静默完成认证，前端只应在已有可用凭据时生效，否则会在无凭据的情况下反复重试。
ALTER TABLE hosts ADD COLUMN vnc_auto_reconnect INTEGER NOT NULL DEFAULT 0
    CHECK (vnc_auto_reconnect IN (0, 1));

-- auto_reconnect_attempts：自动重连的最大尝试次数。0 = 不重试（等价于关闭，但保留用户
--   显式写下的 0）；上限 100 是防呆值——重连间隔由前端退避，次数再大只会拖长"最后失败"
--   的时间且持续骚扰对端服务端。默认 5。
ALTER TABLE hosts ADD COLUMN vnc_auto_reconnect_attempts INTEGER NOT NULL DEFAULT 5
    CHECK (vnc_auto_reconnect_attempts BETWEEN 0 AND 100);
