#!/usr/bin/env node

import {
  closeSync,
  lstatSync,
  openSync,
  readFileSync,
  readSync,
  readdirSync,
  realpathSync,
} from "node:fs";
import {
  basename,
  extname,
  isAbsolute,
  join,
  relative,
  resolve,
  sep,
} from "node:path";
import { fileURLToPath } from "node:url";

import { runCommand } from "../release.mjs";

const MACH_O_MAGIC_BIG_ENDIAN = new Set([
  0xcafebabe, 0xcafebabf, 0xfeedface, 0xfeedfacf, 0xbebafeca, 0xbfbafeca,
  0xcefaedfe, 0xcffaedfe,
]);

const MACH_O_MAGIC_LITTLE_ENDIAN = new Set([
  0xbebafeca, 0xbfbafeca, 0xcefaedfe, 0xcffaedfe, 0xcafebabe, 0xcafebabf,
  0xfeedface, 0xfeedfacf,
]);

const NESTED_CODE_BUNDLE_EXTENSIONS = new Set([
  ".app",
  ".appex",
  ".bundle",
  ".framework",
  ".kext",
  ".plugin",
  ".systemextension",
  ".xpc",
]);

const SIGN_ARGUMENTS = Object.freeze([
  "--force",
  "--sign",
  "-",
  "--timestamp=none",
]);
const VERIFY_ARGUMENTS = Object.freeze([
  "--verify",
  "--deep",
  "--strict",
  "--verbose=2",
]);

function formatPathForError(value) {
  return JSON.stringify(value);
}

function isPathInside(rootPath, candidatePath) {
  const pathFromRoot = relative(rootPath, candidatePath);
  return (
    pathFromRoot === "" ||
    (!pathFromRoot.startsWith(`..${sep}`) &&
      pathFromRoot !== ".." &&
      !isAbsolute(pathFromRoot))
  );
}

function readMachOMagic(filePath) {
  const descriptor = openSync(filePath, "r");
  try {
    const header = Buffer.alloc(4);
    const bytesRead = readSync(descriptor, header, 0, header.length, 0);
    if (bytesRead !== header.length) return undefined;
    return {
      bigEndian: header.readUInt32BE(0),
      littleEndian: header.readUInt32LE(0),
    };
  } finally {
    closeSync(descriptor);
  }
}

export function isMachOFile(filePath) {
  const magic = readMachOMagic(filePath);
  if (!magic) return false;
  return (
    MACH_O_MAGIC_BIG_ENDIAN.has(magic.bigEndian) ||
    MACH_O_MAGIC_LITTLE_ENDIAN.has(magic.littleEndian)
  );
}

function assertInsideBundle(rootPath, targetPath, description) {
  if (!isPathInside(rootPath, targetPath)) {
    throw new Error(
      `${description} points outside the app bundle: ${formatPathForError(targetPath)}`,
    );
  }
}

function validateSymbolicLink(rootPath, linkPath) {
  let resolvedTarget;
  try {
    resolvedTarget = realpathSync(linkPath);
  } catch (error) {
    throw new Error(
      `broken symbolic link in app bundle: ${formatPathForError(linkPath)}`,
      { cause: error },
    );
  }
  assertInsideBundle(
    rootPath,
    resolvedTarget,
    `symbolic link ${formatPathForError(linkPath)}`,
  );
  return resolvedTarget;
}

function readStats(filePath, description) {
  try {
    return lstatSync(filePath);
  } catch (error) {
    throw new Error(
      `${description} is missing or unreadable: ${formatPathForError(filePath)}`,
      { cause: error },
    );
  }
}

function requireDirectory(filePath, description) {
  const stats = readStats(filePath, description);
  if (stats.isSymbolicLink()) {
    throw new Error(
      `${description} must not be a symbolic link: ${formatPathForError(filePath)}`,
    );
  }
  if (!stats.isDirectory()) {
    throw new Error(
      `${description} must be a directory: ${formatPathForError(filePath)}`,
    );
  }
  return stats;
}

function requireRegularFile(
  rootPath,
  filePath,
  description,
  { allowInternalSymlink = true } = {},
) {
  const stats = readStats(filePath, description);
  if (stats.isSymbolicLink()) {
    const resolvedTarget = validateSymbolicLink(rootPath, filePath);
    if (!allowInternalSymlink) {
      throw new Error(
        `${description} must not be a symbolic link: ${formatPathForError(filePath)}`,
      );
    }
    const targetStats = readStats(resolvedTarget, description);
    if (!targetStats.isFile()) {
      throw new Error(
        `${description} must be a regular file: ${formatPathForError(filePath)}`,
      );
    }
    return { path: filePath, realPath: resolvedTarget, stats: targetStats };
  }
  if (!stats.isFile()) {
    throw new Error(
      `${description} must be a regular file: ${formatPathForError(filePath)}`,
    );
  }
  return { path: filePath, realPath: filePath, stats };
}

function decodeXmlEntities(value) {
  const namedEntities = {
    amp: "&",
    apos: "'",
    gt: ">",
    lt: "<",
    quot: '"',
  };
  return value
    .replace(
      /&(#x[0-9a-f]+|#\d+|[a-z]+);/gi,
      (entity, valueWithoutDelimiters) => {
        if (valueWithoutDelimiters.startsWith("#x")) {
          return String.fromCodePoint(
            Number.parseInt(valueWithoutDelimiters.slice(2), 16),
          );
        }
        if (valueWithoutDelimiters.startsWith("#")) {
          return String.fromCodePoint(
            Number.parseInt(valueWithoutDelimiters.slice(1), 10),
          );
        }
        return namedEntities[valueWithoutDelimiters] ?? entity;
      },
    )
    .trim();
}

function readBundleExecutable(infoPlistPath) {
  let plist;
  try {
    plist = readFileSync(infoPlistPath, "utf8");
  } catch (error) {
    throw new Error(
      `Info.plist cannot be read: ${formatPathForError(infoPlistPath)}`,
      { cause: error },
    );
  }

  const match =
    /<key>\s*CFBundleExecutable\s*<\/key>\s*<string(?:\s[^>]*)?>([\s\S]*?)<\/string>/i.exec(
      plist,
    );
  if (!match) {
    throw new Error(
      `Info.plist does not declare CFBundleExecutable: ${formatPathForError(infoPlistPath)}`,
    );
  }

  const executableName = decodeXmlEntities(match[1]);
  if (
    !executableName ||
    executableName === "." ||
    executableName === ".." ||
    basename(executableName) !== executableName
  ) {
    throw new Error(
      `CFBundleExecutable is not a safe executable name: ${formatPathForError(executableName)}`,
    );
  }
  return executableName;
}

function isNestedCodeBundleDirectory(directoryPath) {
  return NESTED_CODE_BUNDLE_EXTENSIONS.has(
    extname(directoryPath).toLowerCase(),
  );
}

function pathDepth(rootPath, targetPath) {
  const pathFromRoot = relative(rootPath, targetPath);
  return pathFromRoot ? pathFromRoot.split(sep).length : 0;
}

function compareDeepestFirst(rootPath, leftPath, rightPath) {
  const depthDifference =
    pathDepth(rootPath, rightPath) - pathDepth(rootPath, leftPath);
  return depthDifference || leftPath.localeCompare(rightPath);
}

function walkBundle(rootPath, directoryPath, state, { isRoot = false } = {}) {
  let entries;
  try {
    entries = readdirSync(directoryPath).sort((left, right) =>
      left.localeCompare(right),
    );
  } catch (error) {
    throw new Error(
      `cannot inspect app bundle directory: ${formatPathForError(directoryPath)}`,
      { cause: error },
    );
  }

  let containsMachO = false;
  for (const entryName of entries) {
    const entryPath = join(directoryPath, entryName);
    const stats = readStats(entryPath, "bundle entry");
    if (stats.isSymbolicLink()) {
      validateSymbolicLink(rootPath, entryPath);
      continue;
    }
    if (stats.isDirectory()) {
      if (walkBundle(rootPath, entryPath, state)) containsMachO = true;
      continue;
    }
    if (!stats.isFile()) continue;

    assertInsideBundle(rootPath, realpathSync(entryPath), "bundle file");
    if (isMachOFile(entryPath)) {
      state.machOFiles.push(entryPath);
      containsMachO = true;
    }
  }

  if (!isRoot && containsMachO && isNestedCodeBundleDirectory(directoryPath)) {
    state.nestedBundles.push(directoryPath);
  }
  return containsMachO;
}

function validateAppRoot(appPath) {
  if (typeof appPath !== "string" || !appPath.trim()) {
    throw new TypeError("appPath must be a non-empty path");
  }

  const requestedPath = resolve(appPath);
  const requestedStats = readStats(requestedPath, "app bundle");
  if (requestedStats.isSymbolicLink()) {
    throw new Error(
      `app root must not be a symbolic link: ${formatPathForError(requestedPath)}`,
    );
  }
  if (!requestedStats.isDirectory()) {
    throw new Error(
      `app root must be a directory: ${formatPathForError(requestedPath)}`,
    );
  }
  if (extname(requestedPath).toLowerCase() !== ".app") {
    throw new Error(
      `app root must end with .app: ${formatPathForError(requestedPath)}`,
    );
  }

  const rootPath = realpathSync(requestedPath);
  assertInsideBundle(rootPath, rootPath, "app root");
  return rootPath;
}

export function inspectApp(appPath) {
  const rootPath = validateAppRoot(appPath);
  const contentsPath = join(rootPath, "Contents");
  requireDirectory(contentsPath, "Contents");

  const infoPlistPath = join(contentsPath, "Info.plist");
  requireRegularFile(rootPath, infoPlistPath, "Info.plist");
  const executableName = readBundleExecutable(infoPlistPath);
  const mainExecutablePath = join(contentsPath, "MacOS", executableName);
  const mainExecutable = requireRegularFile(
    rootPath,
    mainExecutablePath,
    "main executable",
    {
      allowInternalSymlink: false,
    },
  );
  if (!isMachOFile(mainExecutable.realPath)) {
    throw new Error(
      `main executable is not a Mach-O file: ${formatPathForError(mainExecutablePath)}`,
    );
  }

  const state = { machOFiles: [], nestedBundles: [] };
  walkBundle(rootPath, rootPath, state, { isRoot: true });
  if (!state.machOFiles.includes(mainExecutablePath)) {
    throw new Error(
      `main executable was not discovered inside the app bundle: ${formatPathForError(mainExecutablePath)}`,
    );
  }

  const machOFiles = [...new Set(state.machOFiles)].sort((left, right) =>
    compareDeepestFirst(rootPath, left, right),
  );
  const nestedBundles = [...new Set(state.nestedBundles)].sort((left, right) =>
    compareDeepestFirst(rootPath, left, right),
  );
  const signTargets = [...machOFiles, ...nestedBundles, rootPath];

  return Object.freeze({
    appPath: rootPath,
    contentsPath,
    infoPlistPath,
    mainExecutablePath,
    machOFiles: Object.freeze(machOFiles),
    nestedBundles: Object.freeze(nestedBundles),
    signTargets: Object.freeze(signTargets),
  });
}

export function collectSignTargets(appPath) {
  return inspectApp(appPath).signTargets;
}

export async function signApp(
  appPath,
  { run = runCommand, platform = process.platform, signal } = {},
) {
  if (platform !== "darwin") {
    throw new Error(`ad-hoc app signing requires macOS (detected ${platform})`);
  }
  if (typeof run !== "function") {
    throw new TypeError("run must be a function");
  }

  const inspection = inspectApp(appPath);
  for (const targetPath of inspection.signTargets) {
    await run("codesign", [...SIGN_ARGUMENTS, targetPath], { signal });
  }
  await run("codesign", [...VERIFY_ARGUMENTS, inspection.appPath], { signal });
  return inspection;
}

function printUsage() {
  console.error("用法：node build/darwin/sign-app.mjs <path/to/Qterm.app>");
}

if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  const appPath = process.argv[2];
  if (!appPath || process.argv.length !== 3) {
    printUsage();
    process.exitCode = 2;
  } else {
    signApp(appPath)
      .then((inspection) => {
        console.log(`已完成 ad-hoc 签名：${inspection.appPath}`);
      })
      .catch((error) => {
        console.error(`ad-hoc 签名失败：${error.message}`);
        process.exitCode = 1;
      });
  }
}
