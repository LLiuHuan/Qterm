-- 每主机的 X11 转发：是否请求转发，以及可信/不可信模式（可信 = x11-req.single_request=false）。
ALTER TABLE hosts ADD COLUMN x11_forwarding INTEGER NOT NULL DEFAULT 0 CHECK (x11_forwarding IN (0, 1));
ALTER TABLE hosts ADD COLUMN x11_trusted INTEGER NOT NULL DEFAULT 1 CHECK (x11_trusted IN (0, 1));
