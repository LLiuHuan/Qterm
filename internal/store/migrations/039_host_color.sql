-- 主机颜色标签（与快捷命令的 color 同一套语义）。
--
-- 只存 #rrggbb 或空串：颜色只是展示元数据，解析与校验在前端做（`snippetColorStyle` 同款正则），
-- 后端保持"原样存取"，这样旧资产包/旧库升级后行为不变（默认空串 = 不显示颜色）。
ALTER TABLE hosts ADD COLUMN color TEXT NOT NULL DEFAULT '';
