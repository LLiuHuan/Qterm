#!/usr/bin/env node

import { createHash } from "node:crypto";
import {
  createReadStream,
  existsSync,
  mkdirSync,
  readFileSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const SCRIPT_PATH = fileURLToPath(import.meta.url);
const ARCHITECTURES = Object.freeze({
  arm64: Object.freeze({ asset: "qterm-darwin-arm64.zip", cask: "arm" }),
  amd64: Object.freeze({ asset: "qterm-darwin-amd64.zip", cask: "intel" }),
});

function requireValue(value, label) {
  if (typeof value !== "string" || value.length === 0)
    throw new Error(`${label} 不能为空`);
  return value;
}

export function normalizeRepository(raw) {
  const value = requireValue(raw, "GitHub 仓库");
  if (!/^[A-Za-z0-9][A-Za-z0-9._-]*\/[A-Za-z0-9][A-Za-z0-9._-]*$/.test(value)) {
    throw new Error(
      `GitHub 仓库不合法：${JSON.stringify(raw)}（期望 owner/repository）`,
    );
  }
  return value;
}

export function normalizeTag(raw) {
  const value = requireValue(raw, "发布标签");
  if (!/^v\d+\.\d+\.\d+$/.test(value)) {
    throw new Error(`发布标签不合法：${JSON.stringify(raw)}（期望 vX.Y.Z）`);
  }
  return value;
}

function readRegularFile(path, label) {
  if (
    !existsSync(path) ||
    !statSync(path).isFile() ||
    statSync(path).size === 0
  ) {
    throw new Error(`${label} 缺失或为空：${path}`);
  }
  return readFileSync(path);
}

export function parseChecksumManifest(content) {
  const entries = new Map();
  const lines = String(content).split(/\r?\n/);
  for (const [index, line] of lines.entries()) {
    if (line.trim() === "") continue;
    const match = /^([0-9a-fA-F]{64})[ \t]{2,}(.+)$/.exec(line.trimEnd());
    if (!match) throw new Error(`SHA256SUMS 第 ${index + 1} 行格式不合法`);
    const filename = match[2];
    if (entries.has(filename))
      throw new Error(`SHA256SUMS 存在重复资产：${filename}`);
    entries.set(filename, match[1].toLowerCase());
  }
  return entries;
}

async function sha256File(path) {
  if (
    !existsSync(path) ||
    !statSync(path).isFile() ||
    statSync(path).size === 0
  ) {
    throw new Error(`发布资产缺失或为空：${path}`);
  }
  const hash = createHash("sha256");
  for await (const chunk of createReadStream(path)) hash.update(chunk);
  return hash.digest("hex");
}

export async function readReleaseChecksums(artifactsDirectory) {
  const directory = resolve(requireValue(artifactsDirectory, "产物目录"));
  const manifestPath = join(directory, "SHA256SUMS");
  const manifest = parseChecksumManifest(
    readRegularFile(manifestPath, "SHA256SUMS").toString("utf8"),
  );
  const checksums = {};

  for (const [architecture, metadata] of Object.entries(ARCHITECTURES)) {
    const assetPath = join(directory, metadata.asset);
    const listed = manifest.get(metadata.asset);
    if (!listed)
      throw new Error(`SHA256SUMS 缺少 macOS 资产：${metadata.asset}`);
    const actual = await sha256File(assetPath);
    if (actual !== listed) {
      throw new Error(
        `SHA256SUMS 与资产不匹配：${metadata.asset}（清单 ${listed}，实际 ${actual}）`,
      );
    }
    checksums[architecture] = actual;
  }
  return checksums;
}

function rubyQuote(value) {
  return JSON.stringify(value);
}

export function renderCask({ repository, tag, checksums }) {
  const normalizedRepository = normalizeRepository(repository);
  const normalizedTag = normalizeTag(tag);
  for (const architecture of Object.keys(ARCHITECTURES)) {
    if (!/^[0-9a-f]{64}$/.test(checksums?.[architecture] ?? "")) {
      throw new Error(`缺少 ${architecture} 的 SHA256 摘要`);
    }
  }

  const version = normalizedTag.slice(1);
  const releaseURL = `https://github.com/${normalizedRepository}/releases/download/${normalizedTag}`;
  return [
    'cask "qterm" do',
    '  arch arm: "arm64", intel: "amd64"',
    "",
    `  version ${rubyQuote(version)}`,
    "",
    `  sha256 arm:   ${rubyQuote(checksums.arm64)},`,
    `         intel: ${rubyQuote(checksums.amd64)}`,
    "",
    `  url "${releaseURL}/qterm-darwin-#{arch}.zip"`,
    "",
    '  name "Qterm"',
    '  desc "Cross-platform terminal and remote desktop client"',
    `  homepage "https://github.com/${normalizedRepository}"`,
    "",
    "  depends_on macos: :monterey",
    "",
    '  app "qterm.app"',
    "",
    "  caveats do",
    "    <<~EOS",
    "      This Qterm release uses a free ad-hoc code signature and is not notarized by Apple.",
    "      macOS may require allowing Qterm in System Settings > Privacy & Security after installation.",
    "    EOS",
    "  end",
    "end",
    "",
  ].join("\n");
}

export async function writeCask({
  repository,
  tag,
  artifactsDirectory,
  output,
}) {
  const content = renderCask({
    repository,
    tag,
    checksums: await readReleaseChecksums(artifactsDirectory),
  });
  const outputPath = resolve(requireValue(output, "输出文件"));
  mkdirSync(dirname(outputPath), { recursive: true });
  writeFileSync(outputPath, content, "utf8");
  return { outputPath, content };
}

export function parseArguments(argv) {
  const options = {
    repository: undefined,
    tag: undefined,
    artifactsDirectory: undefined,
    output: undefined,
  };
  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index];
    if (argument === "--repository") options.repository = argv[(index += 1)];
    else if (argument.startsWith("--repository="))
      options.repository = argument.slice("--repository=".length);
    else if (argument === "--tag") options.tag = argv[(index += 1)];
    else if (argument.startsWith("--tag="))
      options.tag = argument.slice("--tag=".length);
    else if (argument === "--artifacts")
      options.artifactsDirectory = argv[(index += 1)];
    else if (argument.startsWith("--artifacts="))
      options.artifactsDirectory = argument.slice("--artifacts=".length);
    else if (argument === "--output") options.output = argv[(index += 1)];
    else if (argument.startsWith("--output="))
      options.output = argument.slice("--output=".length);
    else throw new Error(`无法识别的参数：${argument}`);
  }
  normalizeRepository(options.repository);
  normalizeTag(options.tag);
  requireValue(options.artifactsDirectory, "产物目录");
  requireValue(options.output, "输出文件");
  return options;
}

async function main() {
  try {
    const options = parseArguments(process.argv.slice(2));
    const result = await writeCask(options);
    console.log(`[homebrew] Cask 已写入：${result.outputPath}`);
  } catch (error) {
    console.error(`[homebrew] 失败：${error.message}`);
    process.exitCode = 1;
  }
}

if (process.argv[1] && resolve(process.argv[1]) === SCRIPT_PATH) main();
