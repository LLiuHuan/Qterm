-- 凭据提示规则：终端输出命中 pattern 时，由前端弹出凭据列表并由后端注入明文。
-- pattern 在 Go 侧是不透明字符串（不做 regexp.Compile 校验），实际由前端 JS
-- 正则引擎执行；因此这里只约束非空和种类，语法合法性交给前端校验。
CREATE TABLE IF NOT EXISTS credential_prompt_rules (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL CHECK (kind IN ('username','password')),
  pattern TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS credential_prompt_rules_order_idx
  ON credential_prompt_rules(sort_order, id);
