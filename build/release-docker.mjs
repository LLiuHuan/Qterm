#!/usr/bin/env node

import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import {
  existsSync,
  mkdirSync,
  readFileSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { dirname, isAbsolute, join, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

const SCRIPT_DIRECTORY = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(SCRIPT_DIRECTORY, "..");
const IMAGE = process.env.RELEASE_IMAGE || "qterm-release";
const ARTIFACT_DIRECTORY = join(ROOT, "bin", "releases");

export function run(command, args, options = {}) {
  return new Promise((resolveRun, rejectRun) => {
    const child = spawn(command, args, {
      cwd: options.cwd ?? ROOT,
      env: { ...process.env, ...options.env },
      stdio: ["ignore", "inherit", "inherit"],
    });
    child.on("error", rejectRun);
    child.on("close", (code) => {
      if (code === 0) resolveRun();
      else
        rejectRun(
          new Error(`命令失败（${code}）：${command} ${args.join(" ")}`),
        );
    });
  });
}

function readVersion() {
  const config = readFileSync(join(ROOT, "build", "config.yml"), "utf8");
  const matched = /^\s{2}version:\s*"([^"]+)"/m.exec(config);
  if (!matched) throw new Error("无法从 build/config.yml 读取 info.version");
  return matched[1];
}

function timestamp() {
  return new Date()
    .toISOString()
    .replace(/[-:.TZ]/g, "")
    .slice(0, 14);
}

export function writeChecksums(directory, names) {
  const lines = names

    .filter((name) => name !== "SHA256SUMS" && !name.endsWith(".sha256"))
    .sort()
    .map((name) => {
      const digest = createHash("sha256")
        .update(readFileSync(join(directory, name)))
        .digest("hex");
      return `${digest}  ${name}`;
    });
  const path = join(directory, "SHA256SUMS");
  writeFileSync(path, `${lines.join("\n")}\n`, "utf8");
  return path;
}

export function parseArguments(argv) {
  const options = {
    targets: "darwin,windows,linux",
    dryRun: false,
    skipFrontend: false,
    skipDmg: false,
    skipImage: false,
    out: undefined,
  };
  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index];
    if (argument === "--dry-run") options.dryRun = true;
    else if (argument === "--skip-frontend") options.skipFrontend = true;
    else if (argument === "--skip-dmg") options.skipDmg = true;
    else if (argument === "--skip-image") options.skipImage = true;
    else if (argument === "--targets") options.targets = argv[(index += 1)];
    else if (argument.startsWith("--targets="))
      options.targets = argument.slice("--targets=".length);
    else if (argument === "--out") options.out = resolve(argv[(index += 1)]);
    else throw new Error(`无法识别的参数：${argument}`);
  }
  const targets = options.targets?.split(",") ?? [];
  if (
    !targets.length ||
    targets.some((target) => !["darwin", "windows", "linux"].includes(target))
  ) {
    throw new Error(`--targets 不合法：${options.targets}`);
  }
  if (new Set(targets).size !== targets.length)
    throw new Error("--targets 不能重复");
  if (options.out) relativeToRoot(options.out);
  return options;
}

export function validateFrontendBuild(root = ROOT) {
  const index = join(root, "frontend", "dist", "index.html");
  if (
    !existsSync(index) ||
    !statSync(index).isFile() ||
    statSync(index).size === 0
  ) {
    throw new Error(
      "缺少有效的 frontend/dist/index.html，请先执行 npm --prefix frontend run build",
    );
  }
}

export function expectedArtifacts(options, architecture = process.arch) {
  const names = [];
  const targets = options.targets.split(",");
  if (targets.includes("darwin")) {
    for (const arch of ["amd64", "arm64"]) {
      names.push(`qterm-darwin-${arch}.zip`);
      if (!options.skipDmg) names.push(`qterm-darwin-${arch}.dmg`);
    }
  }
  if (targets.includes("windows")) {
    for (const arch of ["amd64", "arm64"])
      names.push(
        `qterm-windows-${arch}.zip`,
        `qterm-windows-${arch}-setup.exe`,
      );
  }
  if (targets.includes("linux")) {
    const arch = architecture === "x64" ? "amd64" : architecture;
    if (!["amd64", "arm64"].includes(arch))
      throw new Error(`不支持的 Linux 发布架构：${architecture}`);
    for (const extension of ["deb", "rpm", "AppImage", "tar.gz"])
      names.push(`qterm-linux-${arch}.${extension}`);
  }
  return names.sort();
}

export function validateArtifacts(
  directory,
  options,
  architecture = process.arch,
) {
  const names = expectedArtifacts(options, architecture);
  for (const name of names) {
    const file = join(directory, name);
    if (
      !existsSync(file) ||
      !statSync(file).isFile() ||
      statSync(file).size === 0
    ) {
      throw new Error(`发布产物缺失或为空：${name}`);
    }
  }
  return names;
}

async function main() {
  const options = parseArguments(process.argv.slice(2));
  const version = readVersion();
  const outDirectory =
    options.out ?? join(ARTIFACT_DIRECTORY, `${timestamp()}-v${version}`);

  const dockerRun = [
    "run",
    "--rm",
    "-v",
    `${ROOT}:/app`,
    "-v",
    `${join(ROOT, "build", "docker", "package.sh")}:/usr/local/bin/package.sh:ro`,
    "-e",
    `TARGETS=${options.targets}`,
    "-e",
    `APP_NAME=qterm`,
    "-e",
    `OUT=/app/${relativeToRoot(outDirectory)}`,
    ...(options.skipDmg ? ["-e", "SKIP_DMG=1"] : []),
    IMAGE,
  ];
  const frontendCommand = ["--prefix", "frontend", "run", "build"];

  if (options.dryRun) {
    console.log(`[release-docker] 版本：${version}`);
    console.log(`[release-docker] 产物目录：${outDirectory}`);
    if (!options.skipFrontend)
      console.log(`[release-docker] 前端：npm ${frontendCommand.join(" ")}`);
    if (options.skipImage) console.log(`[release-docker] 复用镜像：${IMAGE}`);
    else
      console.log(
        `[release-docker] 镜像：docker build -f build/docker/Dockerfile.release -t ${IMAGE} .`,
      );
    console.log(`[release-docker] 打包：docker ${dockerRun.join(" ")}`);
    console.log("[release-docker] 随后写入 SHA256SUMS");
    return;
  }

  if (!options.skipFrontend) {
    console.log("[release-docker] 构建前端（产物被 go:embed 打进二进制）");
    await run("npm", frontendCommand);
  }
  validateFrontendBuild();

  console.log(`[release-docker] 校验版本戳（${version}）`);
  await run("node", [
    join(ROOT, "build", "stamp-version.mjs"),
    version,
    "--check",
  ]);

  if (!options.skipImage) {
    if (!(await imageExists("wails-cross:latest"))) {
      console.log("[release-docker] 构建基础交叉镜像 wails-cross:latest");
      await run("docker", [
        "build",
        "-f",
        join(ROOT, "build", "docker", "Dockerfile.cross"),
        "-t",
        "wails-cross:latest",
        join(ROOT, "build", "docker"),
      ]);
    }
    console.log(`[release-docker] 构建打包镜像 ${IMAGE}`);
    await run("docker", [
      "build",
      "-f",
      join(ROOT, "build", "docker", "Dockerfile.release"),
      "-t",
      IMAGE,
      ROOT,
    ]);
  }

  mkdirSync(outDirectory, { recursive: true });
  console.log(`[release-docker] 在容器内打包（targets=${options.targets}）`);
  await run("docker", dockerRun);

  await run("docker", [
    "run",
    "--rm",
    "-v",
    `${ROOT}:/app`,
    "alpine",
    "chown",
    "-R",
    `${process.getuid()}:${process.getgid()}`,
    `/app/${relativeToRoot(outDirectory)}`,
  ]);

  const names = validateArtifacts(outDirectory, options);
  const checksumPath = writeChecksums(outDirectory, names);
  console.log(`[release-docker] 产物 ${names.length} 个：`);
  for (const name of names.sort()) console.log(`  ${name}`);
  console.log(`[release-docker] 校验和：${checksumPath}`);
}

async function imageExists(tag) {
  return new Promise((resolveExists) => {
    const child = spawn("docker", ["image", "inspect", tag], {
      stdio: "ignore",
    });
    child.on("error", () => resolveExists(false));
    child.on("close", (code) => resolveExists(code === 0));
  });
}

function relativeToRoot(target) {
  const result = relative(ROOT, target);
  if (
    !result ||
    result === ".." ||
    result.startsWith(`..${sep}`) ||
    isAbsolute(result)
  ) {
    throw new Error("--out 必须是项目目录内的子目录，以便挂载给打包容器");
  }
  return result.split(sep).join("/");
}

if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  main().catch((error) => {
    console.error(`[release-docker] 失败：${error.message}`);
    process.exitCode = 1;
  });
}
