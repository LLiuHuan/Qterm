-- 主机标签：JSON 字符串数组（与 environment_variables 同口径），空串 = 没有标签。
-- 标签是非秘密元数据，仅用于检索与展示，因此与主机其它列同级保存、不进加密载荷。
-- 数量与长度上限、去重规则由 model.NormalizeHostTags 统一保证（表单/导入/接口三条路共用一份判据）。
ALTER TABLE hosts ADD COLUMN tags TEXT NOT NULL DEFAULT '';
