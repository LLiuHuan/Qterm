#!/usr/bin/env node

import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const SCRIPT_DIRECTORY = dirname(fileURLToPath(import.meta.url));
const DEFAULT_ROOT = resolve(SCRIPT_DIRECTORY, "..");

export function normalizeVersion(raw) {
  const value = String(raw ?? "")
    .trim()
    .replace(/^v/i, "");
  if (!/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/.test(value)) {
    throw new Error(
      `版本号不合法：${JSON.stringify(raw)}（期望形如 1.2.3 或 1.2.3-rc.1）`,
    );
  }
  return value;
}

function replaceLine(content, file, pattern, replacement) {
  if (!pattern.test(content))
    throw new Error(`无法在 ${file} 里匹配到目标行：${pattern}`);
  return content.replace(pattern, replacement);
}

function readRequired(root, relativePath) {
  const absolute = join(root, relativePath);
  if (!existsSync(absolute))
    throw new Error(`缺少文件：${relativePath}（root=${root}）`);
  return readFileSync(absolute, "utf8");
}

export function stampContent(version) {
  return [
    {
      path: "build/config.yml",

      transform: (text) =>
        replaceLine(
          text,
          "build/config.yml",
          /^(\s{2}version:\s*)"[^"]*"(\s*#.*)?$/m,
          `$1"${version}"$2`,
        ),
    },
    {
      path: "build/darwin/Info.plist",
      transform: (text) =>
        replaceLine(
          text,
          "build/darwin/Info.plist",
          /(<key>CFBundleShortVersionString<\/key>\s*<string>)[^<]*(<\/string>)/,
          `$1${version}$2`,
        ).replace(
          /(<key>CFBundleVersion<\/key>\s*<string>)[^<]*(<\/string>)/,
          `$1${version}$2`,
        ),
    },
    {
      path: "build/linux/nfpm/nfpm.yaml",
      transform: (text) =>
        replaceLine(
          text,
          "build/linux/nfpm/nfpm.yaml",
          /^version:\s*.*$/m,
          `version: "${version}"`,
        ),
    },
    {
      path: "build/windows/info.json",
      transform: (text) =>
        text
          .replace(/("file_version":\s*")[^"]*(")/, `$1${version}$2`)
          .replace(/("ProductVersion":\s*")[^"]*(")/, `$1${version}$2`),
    },
    {
      path: "build/windows/nsis/wails_tools.nsh",
      transform: (text) =>
        replaceLine(
          text,
          "build/windows/nsis/wails_tools.nsh",
          /(!ifndef INFO_PRODUCTVERSION\s*\n\s*!define INFO_PRODUCTVERSION ")[^"]*(")/,
          `$1${version}$2`,
        ),
    },
  ];
}

export function stampVersion({
  root = DEFAULT_ROOT,
  version,
  check = false,
} = {}) {
  const normalized = normalizeVersion(version);
  const changed = [];
  const unchanged = [];
  const mismatched = [];
  for (const file of stampContent(normalized)) {
    const original = readRequired(root, file.path);
    const next = file.transform(original);
    if (next === original) {
      unchanged.push(file.path);
      continue;
    }
    if (check) {
      mismatched.push(file.path);
      continue;
    }
    writeFileSync(join(root, file.path), next, "utf8");
    changed.push(file.path);
  }
  return { version: normalized, changed, unchanged, mismatched };
}

function parseArguments(argv) {
  const options = { version: undefined, root: DEFAULT_ROOT, check: false };
  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index];
    if (argument === "--check") options.check = true;
    else if (argument === "--root") options.root = resolve(argv[(index += 1)]);
    else if (argument.startsWith("--root="))
      options.root = resolve(argument.slice("--root=".length));
    else if (!argument.startsWith("-") && !options.version)
      options.version = argument;
    else throw new Error(`无法识别的参数：${argument}`);
  }
  return options;
}

function main() {
  try {
    const options = parseArguments(process.argv.slice(2));
    if (!options.version)
      throw new Error(
        "用法：node build/stamp-version.mjs <版本号> [--root <仓库根>] [--check]",
      );
    const result = stampVersion(options);
    if (options.check) {
      if (result.mismatched.length > 0) {
        console.error(
          `[stamp-version] 以下文件仍是旧版本（期望 ${result.version}）：${result.mismatched.join(", ")}`,
        );
        process.exitCode = 1;
        return;
      }
      console.log(
        `[stamp-version] 版本一致：${result.version}（${result.unchanged.length} 个文件）`,
      );
      return;
    }
    console.log(
      `[stamp-version] ${result.version}：写入 ${result.changed.length} 个文件` +
        `${result.changed.length ? `（${result.changed.join(", ")}）` : ""}；` +
        `已是该版本 ${result.unchanged.length} 个`,
    );
  } catch (error) {
    console.error(`[stamp-version] 失败：${error.message}`);
    process.exitCode = 1;
  }
}

if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
)
  main();
