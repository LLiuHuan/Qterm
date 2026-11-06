-- 每主机 VNC 的传输维度（承接 034/035 的协议级与界面级选项）。
--
-- 背景（ADR-018）：034 起 VNC 只有一个传输实现——Go 侧裸 TCP 字节桥，把 RFB 字节
-- 原样对接给 WebView 里的 noVNC。但 KasmVNC 这类服务端**根本不提供裸 RFB 端口**：
-- Xvnc 带 `-sslOnly` 启动，只在 HTTPS/WSS 上开一个 WebSocket 端点，RFB 被封在
-- WebSocket 帧里。实测裸 TCP 连它的端口收不到任何 RFB 横幅（服务端在等 TLS
-- ClientHello）。因此新增一个传输维度，默认值保持既有行为不变。
--
-- transport：RFB 字节怎么到服务端。
--   tcp = 裸 RFB 字节（默认，与 034 起的既有行为完全一致，不受本次改动影响）；
--   wss = RFB 字节封在 WebSocket over TLS 里（KasmVNC 及其 websockify 前身）。
--   用 CHECK 而不是自由文本：传输方式决定认证发生在哪一层（见 ws_path 注释），
--   写进一个实现不认识的取值只会变成"连不上但没人知道为什么"。
ALTER TABLE hosts ADD COLUMN vnc_transport TEXT NOT NULL DEFAULT 'tcp'
    CHECK (vnc_transport IN ('tcp', 'wss'));

-- ws_path：只在 transport='wss' 时有意义，是 WebSocket 端点路径。
--   默认 /websockify 是 KasmVNC 自带客户端的默认值（www/app/ui.js 的
--   `UI.initSetting('path', 'websockify')`），因此它是**历史默认值**。
--   但要注意：**根路径 `/` 同样可用**（实测对同一台 KasmVNC，`/` 与 `/websockify`
--   都回 101，且 101 后首包都是 `RFB 003.008\n`）。本仓因此**用 `/` 表示"根路径"**：
--   用户把路径留空时由 model 层归一成 `/` 再落库，本列不出现空字符串。
--   为什么必须带 CHECK：Go 侧会把这个字符串拼进 HTTP 请求行，因此它不是"显示偏好"
--   而是**安全输入**（允许绝对 URL / CRLF / ".." 就能把拨号指向别的主机或注入请求头）。
--   仓库的判据是「正常路径上非法值由 model.NewHost 拦下并报校验错误，数据库的
--   CHECK 只兜住历史数据与资产包导入」——与 vnc_scale 同口径，只是这里用非空与
--   长度约束（路径是自由文本，无法穷举成枚举）。
ALTER TABLE hosts ADD COLUMN vnc_ws_path TEXT NOT NULL DEFAULT '/websockify'
    CHECK (vnc_ws_path <> '' AND length(vnc_ws_path) <= 256);

-- allow_insecure_tls：只在 transport='wss' 时有意义，是否跳过服务端证书校验。
--   默认 0（不跳过）。自签证书是 KasmVNC 的**常见**部署（它自己生成 self.pem），
--   因此这个开关必须存在；但它绝不能默认打开、也不能做成全局设置：
--   开启后该主机的 Basic 凭据与整条 RFB 会话都暴露给中间人，属于用户对该台主机的
--   显式取舍（见 ADR-018 的风险节与界面警示要求）。
ALTER TABLE hosts ADD COLUMN vnc_allow_insecure_tls INTEGER NOT NULL DEFAULT 0
    CHECK (vnc_allow_insecure_tls IN (0, 1));
