import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import {
  existsSync,
  closeSync,
  lstatSync,
  mkdirSync,
  mkdtempSync,
  openSync,
  readFileSync,
  readdirSync,
  statSync,
  unlinkSync,
  writeFileSync,
} from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

export const APP_NAME = "qterm";
export const RELEASES_DIRECTORY = join("bin", "releases");
export const RDP_HELPER_DIRECTORY = join(
  "tools",
  "rdp-helper-poc",
  "freerdp",
  "build",
);

const SCRIPT_DIRECTORY = dirname(fileURLToPath(import.meta.url));
export const DEFAULT_ROOT_DIRECTORY = resolve(SCRIPT_DIRECTORY, "..");
export const DEFAULT_CROSS_IMAGE_PREFIX = "qterm-wails-cross";

export const TARGETS = Object.freeze(
  ["darwin", "linux", "windows"].flatMap((platform) =>
    ["amd64", "arm64"].map((arch) => ({
      arch,
      archiveName: `${APP_NAME}-${platform}-${arch}.${platform === "linux" ? "tar.gz" : "zip"}`,
      binaryName: platform === "windows" ? `${APP_NAME}.exe` : APP_NAME,
      platform,
      task: `${platform}:${platform === "darwin" ? "package" : "build"}`,
      targetDirectoryName: `${platform}-${arch}`,
    })),
  ),
);

const REQUIRED_COMMANDS = Object.freeze([
  [
    "go",
    ["version"],
    "Go is required to compile release binaries. Install Go and retry.",
  ],
  [
    "npm",
    ["--version"],
    "npm is required to build the frontend. Install Node.js/npm and retry.",
  ],
  [
    "wails3",
    ["version"],
    "wails3 is required to run the existing build tasks. Install the Wails CLI and retry.",
  ],
  [
    "xcrun",
    ["-f", "lipo"],
    "Xcode command-line tools with lipo are required for architecture checks. Run xcode-select --install and retry.",
  ],
  [
    "docker",
    ["info"],
    "A running Docker daemon is required for Linux cross-builds. Start Docker Desktop and retry.",
  ],
  [
    "tar",
    ["--version"],
    "tar is required to create Linux portable archives. Install tar and retry.",
  ],
  [
    "zip",
    ["-v"],
    "zip is required to create macOS and Windows portable archives. Install zip and retry.",
  ],
]);

function commandText(command, args) {
  return [command, ...args].join(" ");
}

function quoteForDisplay(value) {
  if (/^[A-Za-z0-9_./:@%+=,-]+$/.test(value)) return value;
  return JSON.stringify(value);
}

function commandTextForDisplay(command, args) {
  return [command, ...args].map(quoteForDisplay).join(" ");
}

export class CommandError extends Error {
  constructor(command, args, code, stderr = "") {
    const detail = stderr
      .trim()
      .split("\n")
      .filter(Boolean)
      .slice(-3)
      .join(" ");
    super(
      `Command failed (${code ?? "unknown"}): ${commandText(command, args)}${detail ? ` — ${detail}` : ""}`,
    );
    this.name = "CommandError";
    this.command = command;
    this.args = args;
    this.code = code;
    this.stderr = stderr;
  }
}

export function runCommand(command, args = [], options = {}) {
  if (options.signal?.aborted) {
    return Promise.reject(
      new CommandError(
        command,
        args,
        "aborted",
        "command was cancelled before it started",
      ),
    );
  }
  return new Promise((resolveCommand, rejectCommand) => {
    const child = spawn(command, args, {
      cwd: options.cwd,
      detached: process.platform !== "win32",
      env: options.env,
      stdio: ["ignore", "pipe", "pipe"],
    });
    let stdout = "";
    let stderr = "";
    const maxCapture = 128 * 1024;

    child.stdout.on("data", (chunk) => {
      if (stdout.length < maxCapture)
        stdout += String(chunk).slice(0, maxCapture - stdout.length);
      if (options.forwardOutput !== false) process.stdout.write(chunk);
    });
    child.stderr.on("data", (chunk) => {
      if (stderr.length < maxCapture)
        stderr += String(chunk).slice(0, maxCapture - stderr.length);
      if (options.forwardOutput !== false) process.stderr.write(chunk);
    });
    const abort = () => {
      try {
        if (process.platform !== "win32" && child.pid)
          process.kill(-child.pid, "SIGTERM");
        else child.kill("SIGTERM");
      } catch {
        child.kill("SIGTERM");
      }
    };
    if (options.signal) {
      if (options.signal.aborted) abort();
      else options.signal.addEventListener("abort", abort, { once: true });
    }
    child.on("error", (error) => {
      rejectCommand(
        new CommandError(command, args, error.code ?? "spawn", error.message),
      );
    });
    child.on("close", (code, signal) => {
      options.signal?.removeEventListener("abort", abort);
      if (options.signal?.aborted) {
        rejectCommand(
          new CommandError(
            command,
            args,
            "aborted",
            stderr || "command was cancelled",
          ),
        );
        return;
      }
      if (code === 0) {
        resolveCommand({ code, stderr, stdout });
        return;
      }
      rejectCommand(new CommandError(command, args, code ?? signal, stderr));
    });
  });
}

function isTruthy(value) {
  return value === true || value === "true" || value === "1" || value === "yes";
}

function toRelativeTaskPath(rootDirectory, value) {
  const absolute = resolve(value);
  const taskPath = relative(rootDirectory, absolute);
  return taskPath || ".";
}

function targetArtifactPath(target, targetDirectory) {
  return join(targetDirectory, target.binaryName);
}

export function makeReleasePlan({
  rootDirectory = DEFAULT_ROOT_DIRECTORY,
  runDirectory,
  crossImagePrefix = DEFAULT_CROSS_IMAGE_PREFIX,
} = {}) {
  const root = resolve(rootDirectory);
  const runDir = resolve(
    runDirectory ?? join(root, RELEASES_DIRECTORY, "planned-run"),
  );
  const targets = TARGETS.map((target) => {
    const targetDirectory = join(runDir, target.targetDirectoryName);
    const binDirectory = toRelativeTaskPath(root, targetDirectory);
    const output = toRelativeTaskPath(
      root,
      targetArtifactPath(target, targetDirectory),
    );
    const vars = [
      `APP_NAME=${APP_NAME}`,
      `BIN_DIR=${binDirectory}`,
      `OUTPUT=${output}`,
      `ARCH=${target.arch}`,
      "DEV=false",
    ];
    if (target.platform === "windows") {
      vars.push("CGO_ENABLED=0");
    }
    if (target.platform === "linux") {
      vars.push(`CROSS_IMAGE=${crossImagePrefix}:${target.arch}`);
    }
    return {
      ...target,
      archivePath: join(runDir, target.archiveName),
      buildCommand: ["wails3", ["task", target.task, ...vars]],
      outputPath: targetArtifactPath(target, targetDirectory),
      targetDirectory,
    };
  });
  return {
    archiveDirectory: runDir,
    checksumPath: join(runDir, "SHA256SUMS"),
    dockerCommands: ["amd64", "arm64"].map((arch) => [
      "docker",
      [
        "build",
        "--platform",
        `linux/${arch}`,
        "--tag",
        `${crossImagePrefix}:${arch}`,
        "--file",
        join(root, "build", "docker", "Dockerfile.cross"),
        join(root, "build", "docker"),
      ],
    ]),
    runDirectory: runDir,
    targets,
  };
}

function parseLipoArchitectures(output) {
  const architectures = new Set();
  for (const token of output.match(/\b(?:arm64|x86_64|amd64|i386|arm64e)\b/g) ??
    []) {
    architectures.add(token === "amd64" ? "x86_64" : token);
  }
  return architectures;
}

function expectedLipoArchitecture(arch) {
  return arch === "amd64" ? "x86_64" : arch;
}

function helperFiles(rootDirectory) {
  const directory = join(rootDirectory, RDP_HELPER_DIRECTORY);
  if (!existsSync(directory))
    return {
      directory,
      helper: join(directory, "qterm-freerdp-helper"),
      dylibs: [],
    };
  const dylibs = readdirSync(directory)
    .filter((name) => /^(?:libfreerdp|libwinpr).*\.dylib$/.test(name))
    .sort()
    .map((name) => join(directory, name));
  return { directory, helper: join(directory, "qterm-freerdp-helper"), dylibs };
}

export async function inspectRdpHelper({
  arch,
  rootDirectory = DEFAULT_ROOT_DIRECTORY,
  lipoCommand = "lipo",
  run = runCommand,
  signal,
} = {}) {
  const files = helperFiles(resolve(rootDirectory));
  if (!existsSync(files.helper)) {
    return {
      include: false,
      reason: `helper is absent at ${files.helper}`,
      files: [],
    };
  }
  if (files.dylibs.length === 0) {
    return {
      include: false,
      reason: `helper dependencies are absent in ${files.directory}`,
      files: [files.helper],
    };
  }

  const requiredArchitecture = expectedLipoArchitecture(arch);
  const inspectedFiles = [files.helper, ...files.dylibs];
  for (const file of inspectedFiles) {
    let result;
    try {
      result = await run(lipoCommand, ["-archs", file], {
        forwardOutput: false,
        signal,
      });
    } catch (error) {
      return {
        include: false,
        reason: `lipo could not inspect ${file}: ${error.message}`,
        files: inspectedFiles,
      };
    }
    const architectures = parseLipoArchitectures(result.stdout);
    if (!architectures.has(requiredArchitecture)) {
      const available = [...architectures].sort().join(", ") || "unknown";
      return {
        include: false,
        reason: `${file} does not contain ${requiredArchitecture} (contains ${available})`,
        files: inspectedFiles,
      };
    }
  }

  return {
    include: true,
    reason: `helper and ${files.dylibs.length} bundled dylib(s) contain ${requiredArchitecture}`,
    files: inspectedFiles,
  };
}

export async function runPreflight({
  platform = process.platform,
  run = runCommand,
  signal,
} = {}) {
  if (platform !== "darwin") {
    throw new Error(
      `release:all must run on macOS (detected ${platform}). Run it from a macOS host.`,
    );
  }

  let lipoCommand = "lipo";
  for (const [command, args, message] of REQUIRED_COMMANDS) {
    try {
      const result = await run(command, args, { forwardOutput: false, signal });
      if (command === "xcrun") {
        lipoCommand =
          result.stdout.trim().split("\n").find(Boolean) || lipoCommand;
      }
    } catch (error) {
      throw new Error(
        `${message}\n  Check: ${commandTextForDisplay(command, args)}\n  ${error.message}`,
      );
    }
  }
  return { lipoCommand };
}

function assertExistingFile(file, description) {
  if (!existsSync(file) || !statSync(file).isFile()) {
    throw new Error(`${description} was not produced: ${file}`);
  }
}

async function archiveTarget(target, run) {
  assertExistingFile(
    target.outputPath,
    `${target.platform}/${target.arch} build output`,
  );
  const archiveArguments =
    target.platform === "linux"
      ? ["-czf", target.archivePath, target.binaryName]
      : [
          "-qry",
          target.archivePath,
          target.platform === "darwin" ? `${APP_NAME}.app` : target.binaryName,
        ];
  const command = target.platform === "linux" ? "tar" : "zip";
  await run(command, archiveArguments, { cwd: target.targetDirectory });
  assertExistingFile(
    target.archivePath,
    `${target.platform}/${target.arch} archive`,
  );
}

function sha256(file) {
  return createHash("sha256").update(readFileSync(file)).digest("hex");
}

function writeChecksums(plan) {
  const lines = plan.targets.map(
    (target) => `${sha256(target.archivePath)}  ${target.archiveName}`,
  );
  writeFileSync(plan.checksumPath, `${lines.join("\n")}\n`, {
    encoding: "utf8",
    flag: "wx",
  });
}

function printDryRun(plan, logger) {
  logger(
    "[release] DRY_RUN=true; no preflight, build, archive, lock, or filesystem writes will run.",
  );
  logger("[release] preflight:");
  for (const [command, args] of REQUIRED_COMMANDS)
    logger(`  ${commandTextForDisplay(command, args)}`);
  for (const [command, args] of plan.dockerCommands)
    logger(`  ${commandTextForDisplay(command, args)}`);
  for (const target of plan.targets) {
    logger(`  ${commandTextForDisplay(...target.buildCommand)}`);
    const archiveArguments =
      target.platform === "linux"
        ? ["-czf", target.archivePath, target.binaryName]
        : [
            "-qry",
            target.archivePath,
            target.platform === "darwin"
              ? `${APP_NAME}.app`
              : target.binaryName,
          ];
    logger(
      `  ${commandTextForDisplay(target.platform === "linux" ? "tar" : "zip", archiveArguments)}`,
    );
  }
  logger(`  write ${plan.checksumPath}`);
  logger(`[release] planned output directory: ${plan.runDirectory}`);
}

function acquireLock(releasesDirectory) {
  const lockPath = join(releasesDirectory, ".release.lock");
  let descriptor;
  try {
    descriptor = openSync(lockPath, "wx");
  } catch (error) {
    if (error.code === "EEXIST") {
      throw new Error(
        `Another release is already running (lock: ${lockPath}). Remove it only after confirming no release process remains.`,
      );
    }
    throw error;
  }
  writeFileSync(descriptor, `${process.pid}\n`, { encoding: "utf8" });
  closeSync(descriptor);
  return lockPath;
}

function ensureDirectoryIsWritable(directory) {
  if (!existsSync(directory)) return;
  const stats = lstatSync(directory);
  if (!stats.isDirectory())
    throw new Error(`Release output root is not a directory: ${directory}`);
}

function assertNoSharedIntermediate(rootDirectory, target) {
  const sharedPath =
    target.platform === "linux"
      ? join(rootDirectory, "bin", `${APP_NAME}-linux-${target.arch}`)
      : target.platform === "windows"
        ? join(rootDirectory, `wails_windows_${target.arch}.syso`)
        : null;
  if (sharedPath && existsSync(sharedPath)) {
    throw new Error(
      `Refusing to overwrite existing build intermediate ${sharedPath}. Move it aside manually, then retry release:all.`,
    );
  }
}

export async function runRelease({
  rootDirectory = DEFAULT_ROOT_DIRECTORY,
  dryRun = isTruthy(process.env.DRY_RUN),
  platform = process.platform,
  crossImagePrefix = DEFAULT_CROSS_IMAGE_PREFIX,
  run = runCommand,
  preflight = runPreflight,
  logger = console.log,
} = {}) {
  const root = resolve(rootDirectory);
  const plannedRunDirectory = join(root, RELEASES_DIRECTORY, "planned-run");
  if (dryRun) {
    const plan = makeReleasePlan({
      crossImagePrefix,
      rootDirectory: root,
      runDirectory: plannedRunDirectory,
    });
    printDryRun(plan, logger);
    return { dryRun: true, plan };
  }

  const controller = new AbortController();
  const onSignal = (signal) => {
    logger(
      `[release] received ${signal}; stopping the active child command and retaining partial output.`,
    );
    controller.abort();
  };
  const onSigint = () => onSignal("SIGINT");
  const onSigterm = () => onSignal("SIGTERM");
  process.once("SIGINT", onSigint);
  process.once("SIGTERM", onSigterm);
  let lockPath;
  try {
    const tools = await preflight({ platform, run, signal: controller.signal });
    const releasesDirectory = join(root, RELEASES_DIRECTORY);
    ensureDirectoryIsWritable(releasesDirectory);
    mkdirSync(releasesDirectory, { recursive: true });
    lockPath = acquireLock(releasesDirectory);
    let plan;
    try {
      plan = makeReleasePlan({
        crossImagePrefix,
        rootDirectory: root,
        runDirectory: mkdtempSync(
          join(
            releasesDirectory,
            `${new Date().toISOString().replace(/[-:.TZ]/g, "")}-`,
          ),
        ),
      });
      logger(`[release] output directory: ${plan.runDirectory}`);

      for (const [command, args] of plan.dockerCommands) {
        logger(`[release] ${commandTextForDisplay(command, args)}`);
        await run(command, args, { cwd: root, signal: controller.signal });
      }

      for (const target of plan.targets) {
        const vars = target.buildCommand[1].slice(2);
        if (target.platform === "darwin") {
          const helper = await inspectRdpHelper({
            arch: target.arch,
            lipoCommand: tools?.lipoCommand ?? "lipo",
            rootDirectory: root,
            run,
            signal: controller.signal,
          });
          if (!helper.include) {
            logger(
              `[release] darwin/${target.arch}: omitting RDP helper: ${helper.reason}`,
            );
          } else {
            logger(`[release] darwin/${target.arch}: ${helper.reason}`);
          }
          vars.push(`SKIP_RDP_HELPER=${helper.include ? "false" : "true"}`);
        }
        assertNoSharedIntermediate(root, target);
        mkdirSync(target.targetDirectory, { recursive: true });
        const [command, args] = [
          target.buildCommand[0],
          ["task", target.task, ...vars],
        ];
        logger(`[release] ${commandTextForDisplay(command, args)}`);
        await run(command, args, { cwd: root, signal: controller.signal });
        assertExistingFile(
          target.outputPath,
          `${target.platform}/${target.arch} build output`,
        );
      }

      for (const target of plan.targets) {
        logger(`[release] archiving ${target.archiveName}`);
        await archiveTarget(target, (command, args, options = {}) =>
          run(command, args, { ...options, signal: controller.signal }),
        );
      }
      writeChecksums(plan);
      logger(`[release] SHA256SUMS written: ${plan.checksumPath}`);
      return { dryRun: false, plan };
    } catch (error) {
      if (plan)
        error.message = `${error.message}\nPartial release output retained at ${plan.runDirectory}.`;
      throw error;
    } finally {
      if (lockPath) unlinkSync(lockPath);
    }
  } finally {
    process.removeListener("SIGINT", onSigint);
    process.removeListener("SIGTERM", onSigterm);
  }
}

async function main() {
  try {
    await runRelease();
  } catch (error) {
    console.error(`[release] failed: ${error.message}`);
    process.exitCode = 1;
  }
}

if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  await main();
}
