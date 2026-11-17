#!/usr/bin/env node

import {
  createHash,
  sign,
  verify,
  createPrivateKey,
  createPublicKey,
} from "node:crypto";
import { readdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";

const ARTIFACT_PREFIX = "qterm-";
const CHECKSUM_NAME = "SHA256SUMS";
const SIGNATURE_NAME = "SHA256SUMS.sig";

function decodeKey(value) {
  const trimmed = value.trim();
  if (/^[0-9a-fA-F]+$/.test(trimmed) && trimmed.length % 2 === 0) {
    return Buffer.from(trimmed, "hex");
  }
  return Buffer.from(trimmed, "base64");
}

function privateKeyFrom(body) {
  if (body.length === 64) {
    return createPrivateKey({
      key: Buffer.concat([
        Buffer.from("302e020100300506032b657004220420", "hex"),
        body.subarray(0, 32),
      ]),
      format: "der",
      type: "pkcs8",
    });
  }
  if (body.length === 32) {
    return createPrivateKey({
      key: Buffer.concat([
        Buffer.from("302e020100300506032b657004220420", "hex"),
        body,
      ]),
      format: "der",
      type: "pkcs8",
    });
  }
  throw new Error(`私钥长度不对：期望 32 或 64 字节，实际 ${body.length}`);
}

function publicKeyFrom(body) {
  if (body.length !== 32) {
    throw new Error(`公钥长度不对：期望 32 字节，实际 ${body.length}`);
  }
  return createPublicKey({
    key: Buffer.concat([Buffer.from("302a300506032b6570032100", "hex"), body]),
    format: "der",
    type: "spki",
  });
}

export function artifactFiles(directory) {
  return readdirSync(directory)
    .filter((name) => name.startsWith(ARTIFACT_PREFIX))
    .sort();
}

export function buildChecksums(directory) {
  const lines = artifactFiles(directory).map((name) => {
    const body = readFileSync(path.join(directory, name));
    return `${createHash("sha256").update(body).digest("hex")}  ${name}`;
  });
  return `${lines.join("\n")}\n`;
}

export function signChecksums(checksums, privateKeyHexOrBase64) {
  const key = privateKeyFrom(decodeKey(privateKeyHexOrBase64));
  return sign(null, Buffer.from(checksums), key).toString("hex");
}

export function verifyChecksums(
  checksums,
  signatureHexOrBase64,
  publicKeyHexOrBase64,
) {
  const key = publicKeyFrom(decodeKey(publicKeyHexOrBase64));
  return verify(
    null,
    Buffer.from(checksums),
    key,
    decodeKey(signatureHexOrBase64),
  );
}

function main(argv) {
  const args = argv.slice(2);
  const get = (flag) => {
    const index = args.indexOf(flag);
    return index === -1 ? undefined : args[index + 1];
  };
  const directory = get("--artifacts") ?? "artifacts";
  const publicKey = get("--public-key");

  const checksums = buildChecksums(directory);
  const checksumPath = path.join(directory, CHECKSUM_NAME);
  const signaturePath = path.join(directory, SIGNATURE_NAME);

  if (args.includes("--verify")) {
    const signature = args.includes("--from-file")
      ? readFileSync(signaturePath, "utf8")
      : get("--signature");
    if (!signature) throw new Error("--verify 需要 --signature 或 --from-file");
    if (!publicKey) throw new Error("--verify 需要 --public-key");

    const fileChecksums = readFileSync(checksumPath, "utf8");
    if (!verifyChecksums(fileChecksums, signature, publicKey)) {
      throw new Error("清单签名校验失败");
    }

    if (args.includes("--check")) {
      const expected = buildChecksums(directory);
      if (expected !== fileChecksums) {
        throw new Error("清单与当前产物不一致");
      }
    }
    process.stdout.write("[sign-update-manifest] 签名校验通过\n");
    return;
  }

  const privateKey = process.env.QTERM_UPDATE_SIGNING_KEY;
  if (!privateKey) {
    throw new Error(
      "缺少 QTERM_UPDATE_SIGNING_KEY（仓库 secret，Ed25519 私钥）",
    );
  }
  writeFileSync(checksumPath, checksums);
  const signature = signChecksums(checksums, privateKey);
  writeFileSync(signaturePath, `${signature}\n`);
  process.stdout.write(
    `[sign-update-manifest] 已生成 ${CHECKSUM_NAME} 与 ${SIGNATURE_NAME}\n`,
  );
}

if (import.meta.url === `file://${process.argv[1]}`) {
  try {
    main(process.argv);
  } catch (error) {
    process.stderr.write(`[sign-update-manifest] 失败：${error.message}\n`);
    process.exit(1);
  }
}
