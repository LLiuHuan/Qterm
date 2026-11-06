import { execSync } from "node:child_process";
import { readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = path.dirname(fileURLToPath(import.meta.url));

const TYPES = [
  { value: "feat", name: "feat:     新增功能 | A new feature" },
  { value: "fix", name: "fix:      修复缺陷 | A bug fix" },
  { value: "docs", name: "docs:     文档变更 | Documentation only changes" },
  {
    value: "style",
    name: "style:    代码格式 | Changes that do not affect the meaning of the code",
  },
  {
    value: "refactor",
    name: "refactor: 代码重构 | A code change that neither fixes a bug nor adds a feature",
  },
  {
    value: "perf",
    name: "perf:     性能优化 | A code change that improves performance",
  },
  {
    value: "test",
    name: "test:     添加疏漏测试或已有测试改动 | Adding missing tests or correcting existing tests",
  },
  {
    value: "build",
    name: "build:    构建流程、外部依赖变更（如升级 npm 包、修改打包配置等） | Changes that affect the build system or external dependencies",
  },
  {
    value: "ci",
    name: "ci:       修改 CI 配置、脚本 | Changes to our CI configuration files and scripts",
  },
  { value: "revert", name: "revert:   回滚 commit | Revert to a commit" },
  {
    value: "chore",
    name: "chore:    对构建过程或辅助工具和库的更改（不影响源文件、测试用例） | Other changes that do not modify src or test files",
  },
  { value: "wip", name: "wip:      正在开发中 | Under development" },
  { value: "workflow", name: "workflow: 工作流程改进 | Workflow improvement" },
  {
    value: "types",
    name: "types:    类型定义文件修改 | Type definition file modification",
  },
  { value: "release", name: "release:  发布版本 | Release a version" },
];

const MODULE_SCOPES = [
  "project",
  "deps",
  "build",
  "ci",
  "release",
  "docs",
  "scripts",
  "i18n",
  "ui",
  "other",
];

function listDirectories(relativePath) {
  try {
    return readdirSync(path.join(repoRoot, relativePath), {
      withFileTypes: true,
    })
      .filter((entry) => entry.isDirectory())
      .map((entry) => entry.name);
  } catch {
    return [];
  }
}

const FEATURE_SCOPES = [
  ...listDirectories("internal"),
  ...listDirectories("frontend/src/features"),
];

const allowedScopes = [
  ...new Set([...MODULE_SCOPES, ...FEATURE_SCOPES]),
].sort();
const scopeSet = new Set(allowedScopes);

function scopeForPath(relativePath) {
  const segments = relativePath.split("/");
  if (segments[0] === "internal" && segments[1] && scopeSet.has(segments[1]))
    return segments[1];
  if (segments[0] === "frontend") {
    if (segments[1] === "src" && segments[2] === "features" && segments[3]) {
      return scopeSet.has(segments[3]) ? segments[3] : undefined;
    }
    if (segments[1] === "src" && segments[2] === "locales") return "i18n";
    if (
      segments[1] === "src" &&
      (segments[2] === "components" || segments[2] === "styles")
    )
      return "ui";
    if (segments[1] === "package.json" || segments[1] === "package-lock.json")
      return "deps";
    return "ui";
  }
  if (segments[0] === "build") return "build";
  if (segments[0] === "docs") return "docs";
  if (segments[0] === "scripts") return "scripts";
  if (segments[0] === "tools") return "other";
  if (segments[0] === ".github") return "ci";
  if (segments[0] === "go.mod" || segments[0] === "go.sum") return "deps";
  if (segments[0] === ".husky" || segments[0] === "commitlint.config.mjs")
    return "project";
  return undefined;
}

function detectDefaultScope() {
  try {
    const staged = execSync(
      "git diff --cached --name-only --diff-filter=ACMR",
      {
        cwd: repoRoot,
        encoding: "utf8",
        stdio: ["ignore", "pipe", "ignore"],
      },
    )
      .split("\n")
      .filter(Boolean);
    const changed =
      staged.length > 0
        ? staged
        : execSync("git status --porcelain", {
            cwd: repoRoot,
            encoding: "utf8",
            stdio: ["ignore", "pipe", "ignore"],
          })
            .split("\n")
            .filter(Boolean)
            .map((line) => line.slice(3).trim());
    for (const file of changed) {
      const scope = scopeForPath(file);
      if (scope) return scope;
    }
  } catch {
    return undefined;
  }
  return undefined;
}

const defaultScope = detectDefaultScope();

const userConfig = {
  extends: ["@commitlint/config-conventional"],
  plugins: ["commitlint-plugin-function-rules"],
  prompt: {
    alias: {
      b: "build: bump dependencies",
      c: "chore: update config",
      f: "docs: fix typos",
      r: "docs: update README",
      s: "style: update code format",
    },
    allowCustomIssuePrefixs: false,
    allowEmptyIssuePrefixs: false,
    customScopesAlign: defaultScope ? "bottom" : "top",
    defaultScope,
    scopes: allowedScopes,
    types: TYPES,
    messages: {
      type: "选择你要提交的类型 :",
      scope: "选择一个提交范围 (可选):",
      customScope: "请输入自定义的提交范围 :",
      subject: "填写简短精炼的变更描述 :\n",
      body: '填写更加详细的变更描述 (可选)。使用 "|" 换行 :\n',
      breaking: '列举非兼容性重大的变更 (可选)。使用 "|" 换行 :\n',
      footerPrefixsSelect: "选择关联issue前缀 (可选):",
      customFooterPrefixs: "输入自定义issue前缀 :",
      footer: "列举关联issue (可选) 例如: #31, #I3244 :\n",
      confirmCommit: "是否提交或修改commit ?",
    },
    emptyScopesAlias: "empty:      不填写",
    customScopesAlias: "custom:     自定义",
  },
  rules: {
    "body-leading-blank": [2, "always"],

    "footer-leading-blank": [1, "always"],

    "function-rules/scope-enum": [
      2,
      "always",
      (parsed) => {
        if (!parsed.scope || scopeSet.has(parsed.scope)) return [true];
        return [
          false,
          `scope 必须是以下之一（或留空）：${allowedScopes.join(", ")}`,
        ];
      },
    ],

    "header-max-length": [2, "always", 108],

    "scope-enum": [0],

    "subject-case": [0],

    "subject-empty": [2, "never"],

    "type-empty": [2, "never"],

    "type-enum": [2, "always", TYPES.map((type) => type.value)],
  },
};

export default userConfig;
