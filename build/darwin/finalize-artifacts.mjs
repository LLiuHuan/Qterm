#!/usr/bin/env node

import {
  existsSync,
  lstatSync,
  mkdirSync,
  mkdtempSync,
  renameSync,
} from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { runCommand } from "../release.mjs";
import { signApp } from "./sign-app.mjs";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const ARCHITECTURES = ["amd64", "arm64"];

export function validateArchiveEntries(output) {
  const entries = output.split(/\r?\n/).filter(Boolean);
  if (!entries.length) throw new Error("macOS ZIP 为空");
  const seen = new Set();
  for (const entry of entries) {
    const parts = entry.split("/");
    const metadata =
      parts[0] === "__MACOSX" &&
      (parts[1] === "qterm.app" ||
        parts[1] === "._qterm.app" ||
        entry === "__MACOSX/");
    if (
      entry.includes("\\") ||
      parts.includes("..") ||
      parts.includes(".") ||
      (parts[0] !== "qterm.app" && !metadata)
    ) {
      throw new Error(`macOS ZIP 含非预期路径：${entry}`);
    }
    if (seen.has(entry)) throw new Error(`macOS ZIP 含重复路径：${entry}`);
    seen.add(entry);
  }
  if (!seen.has("qterm.app/Contents/MacOS/qterm"))
    throw new Error("macOS ZIP 缺少主程序");
}

function requireFile(file) {
  if (
    !existsSync(file) ||
    !lstatSync(file).isFile() ||
    lstatSync(file).size === 0
  ) {
    throw new Error(`macOS 发布产物缺失或为空：${file}`);
  }
}

export async function verifyArchive(archive, destination, run = runCommand) {
  const listing = await run("unzip", ["-Z1", archive], {
    forwardOutput: false,
  });
  validateArchiveEntries(listing.stdout);
  mkdirSync(destination, { recursive: true });
  await run("ditto", ["-x", "-k", archive, destination]);
  const app = join(destination, "qterm.app");
  await run("codesign", ["--verify", "--deep", "--strict", "--verbose=2", app]);
  return app;
}

export async function verifyDiskImage(dmg, mountpoint, run = runCommand) {
  mkdirSync(mountpoint, { recursive: true });
  await run("hdiutil", ["verify", dmg]);
  await run("hdiutil", [
    "attach",
    "-readonly",
    "-nobrowse",
    "-mountpoint",
    mountpoint,
    dmg,
  ]);
  try {
    await run("codesign", [
      "--verify",
      "--deep",
      "--strict",
      "--verbose=2",
      join(mountpoint, "qterm.app"),
    ]);
  } finally {
    await run("hdiutil", ["detach", mountpoint]);
  }
}

export async function finalizeDarwinArtifacts(
  directory,
  {
    architectures = ARCHITECTURES,
    createDmg = true,
    rootDirectory = ROOT,
    platform = process.platform,
    run = runCommand,
    sign = signApp,
  } = {},
) {
  if (platform !== "darwin") throw new Error("macOS 最终签名必须在 macOS 执行");
  if (
    !architectures.length ||
    new Set(architectures).size !== architectures.length ||
    architectures.some((arch) => !ARCHITECTURES.includes(arch))
  ) {
    throw new Error("macOS 目标架构必须为 amd64 / arm64，且不能重复");
  }
  directory = resolve(directory);
  for (const arch of architectures)
    requireFile(join(directory, `qterm-darwin-${arch}.zip`));
  const work = mkdtempSync(join(directory, ".qterm-adhoc-"));
  const prepared = [];
  for (const arch of architectures) {
    const archiveName = `qterm-darwin-${arch}.zip`;
    const source = join(directory, archiveName);
    const listing = await run("unzip", ["-Z1", source], {
      forwardOutput: false,
    });
    validateArchiveEntries(listing.stdout);
    const stage = join(work, arch);
    mkdirSync(stage);
    await run("ditto", ["-x", "-k", source, stage]);
    const app = join(stage, "qterm.app");
    await run("lipo", [
      join(app, "Contents/MacOS/qterm"),
      "-verify_arch",
      arch === "amd64" ? "x86_64" : "arm64",
    ]);
    await sign(app, { run, platform });

    const signedZip = join(work, archiveName);
    await run("ditto", [
      "-c",
      "-k",
      "--norsrc",
      "--noextattr",
      "--noacl",
      "--keepParent",
      app,
      signedZip,
    ]);
    requireFile(signedZip);
    await verifyArchive(signedZip, join(work, `${arch}-verify`), run);
    prepared.push({ source: signedZip, target: join(directory, archiveName) });

    if (createDmg) {
      await run(
        "wails3",
        [
          "tool",
          "package",
          "--format",
          "dmg",
          "--name",
          "qterm",
          "--out",
          stage,
          "--background",
          join(rootDirectory, "build/darwin/dmg-background.png"),
          "--volume-icon",
          join(rootDirectory, "build/darwin/icons.icns"),
          "--file-icon",
          join(rootDirectory, "build/darwin/dmg-file-icon.icns"),
          "--window-width",
          "540",
          "--window-height",
          "380",
        ],
        { cwd: rootDirectory },
      );
      const dmg = join(stage, "qterm.dmg");
      requireFile(dmg);
      await run("codesign", [
        "--force",
        "--sign",
        "-",
        "--timestamp=none",
        dmg,
      ]);
      await run("codesign", ["--verify", "--strict", "--verbose=2", dmg]);
      await verifyDiskImage(dmg, join(work, `${arch}-mount`), run);
      prepared.push({
        source: dmg,
        target: join(directory, `qterm-darwin-${arch}.dmg`),
      });
    }
  }

  for (const item of prepared) renameSync(item.source, item.target);
  return prepared.map((item) => item.target);
}

async function main() {
  const args = process.argv.slice(2);
  const directory = args.shift();
  if (!directory || args.some((arg) => arg !== "--skip-dmg"))
    throw new Error(
      "用法：node build/darwin/finalize-artifacts.mjs <产物目录> [--skip-dmg]",
    );
  const files = await finalizeDarwinArtifacts(directory, {
    createDmg: !args.includes("--skip-dmg"),
  });
  for (const file of files) console.log(`[adhoc] 已签名并验证：${file}`);
}
if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  main().catch((error) => {
    console.error(`[adhoc] 失败：${error.message}`);
    process.exitCode = 1;
  });
}
