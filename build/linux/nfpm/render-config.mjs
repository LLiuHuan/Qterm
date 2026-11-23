import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

export const HELPER_SRC =
  "./tools/rdp-helper-poc/freerdp/build/qterm-freerdp-helper";
export const HELPER_DST = "/usr/local/bin/qterm-freerdp-helper";
export const CLI_SRC = "./bin/qterm-cli";
export const CLI_DST = "/usr/local/bin/qterm-cli";

export function withRdpHelper(text, present) {
  if (!present) return text;
  const anchor = /^contents:[ \t]*$/m;
  if (!anchor.test(text)) {
    throw new Error("nfpm 配置缺少 contents 段，无法追加 RDP helper");
  }
  return text.replace(
    anchor,
    (line) => `${line}\n  - src: "${HELPER_SRC}"\n    dst: "${HELPER_DST}"`,
  );
}

export function withCLI(text, present) {
  if (!present) return text;
  const anchor = /^contents:[ \t]*$/m;
  if (!anchor.test(text)) {
    throw new Error("nfpm 配置缺少 contents 段，无法追加命令行客户端");
  }
  return text.replace(
    anchor,
    (line) =>
      `${line}\n  - src: "${CLI_SRC}"\n    dst: "${CLI_DST}"\n    file_info:\n      mode: 0755`,
  );
}

function main() {
  const here = dirname(fileURLToPath(import.meta.url));
  const root = resolve(here, "../../..");
  const source = join(here, "nfpm.yaml");
  const target = join(here, "nfpm.generated.yaml");
  const present = existsSync(join(root, HELPER_SRC));
  const cliPresent = existsSync(join(root, CLI_SRC));

  const rendered = withCLI(
    withRdpHelper(readFileSync(source, "utf8"), present),
    cliPresent,
  );
  writeFileSync(target, rendered);
  const note = present
    ? "已包含 RDP helper"
    : "未找到 RDP helper（包内不带 RDP）";
  const cliNote = cliPresent
    ? "已包含命令行客户端"
    : "未找到命令行客户端（包内不带 qterm-cli 命令）";
  console.log(`[nfpm] 生成 ${target}：${note}；${cliNote}`);
}

if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  main();
}
