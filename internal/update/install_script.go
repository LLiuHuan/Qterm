package update

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func darwinBundleFromExecutable(executable string) (string, bool) {
	clean := filepath.Clean(executable)
	macOS := filepath.Dir(clean)
	contents := filepath.Dir(macOS)
	bundle := filepath.Dir(contents)
	if filepath.Base(macOS) != "MacOS" || filepath.Base(contents) != "Contents" {
		return "", false
	}
	if !strings.HasSuffix(bundle, ".app") {
		return "", false
	}
	return bundle, true
}

func darwinReplacerScript() string {
	return `#!/bin/sh
set -u
pid="$1"
bundle="$2"
archive="$3"
while kill -0 "$pid" 2>/dev/null; do
  sleep 0.2
done
parent=$(dirname "$bundle")
backup="$bundle.old"
rm -rf "$backup"
if ! mv "$bundle" "$backup"; then
  # 连备份都做不到（例如 /Applications 需要管理员权限）：别再动现场，直接拉起旧版本。
  open "$bundle" 2>/dev/null || true
  exit 1
fi
if unzip -q -o "$archive" -d "$parent" >/dev/null 2>&1; then
  xattr -dr com.apple.quarantine "$bundle" >/dev/null 2>&1 || true
  open "$bundle" 2>/dev/null || true
  rm -rf "$backup"
else
  rm -rf "$bundle"
  mv "$backup" "$bundle"
  open "$bundle" 2>/dev/null || true
  exit 1
fi
`
}

func linuxReplacerScript() string {
	return `#!/bin/sh
set -u
pid="$1"
target="$2"
archive="$3"
while kill -0 "$pid" 2>/dev/null; do
  sleep 0.2
done
workdir=$(dirname "$archive")
extract="$workdir/extract"
rm -rf "$extract"
mkdir -p "$extract"
if ! tar -xzf "$archive" -C "$extract" >/dev/null 2>&1; then
  exec "$target" >/dev/null 2>&1 || true
  exit 1
fi
binary=$(find "$extract" -type f -name qterm 2>/dev/null | head -n 1)
if [ -z "$binary" ]; then
  rm -rf "$extract"
  exec "$target" >/dev/null 2>&1 || true
  exit 1
fi
staged="$target.new"
if ! cp "$binary" "$staged" || ! chmod +x "$staged" || ! mv "$staged" "$target"; then
  rm -f "$staged"
  rm -rf "$extract"
  exec "$target" >/dev/null 2>&1 || true
  exit 1
fi
rm -rf "$extract"
# 用 setsid 脱离当前会话，父进程退出后仍能留在用户桌面会话里。
setsid "$target" >/dev/null 2>&1 &
`
}

func windowsReplacerScript() string {
	return `param(
  [Parameter(Mandatory = $true)][int]$ProcessId,
  [Parameter(Mandatory = $true)][string]$Target,
  [Parameter(Mandatory = $true)][string]$Archive
)
$ErrorActionPreference = 'Stop'
Wait-Process -Id $ProcessId -ErrorAction SilentlyContinue
$work = Join-Path ([System.IO.Path]::GetTempPath()) ('qterm-update-' + [guid]::NewGuid().ToString('N'))
try {
  New-Item -ItemType Directory -Path $work | Out-Null
  Expand-Archive -Path $Archive -DestinationPath $work -Force
  $binary = Get-ChildItem -Path $work -Recurse -Filter 'qterm.exe' | Select-Object -First 1
  if (-not $binary) { throw 'qterm.exe not found in archive' }
  Copy-Item -LiteralPath $binary.FullName -Destination $Target -Force
  Start-Process -FilePath $Target
} catch {
  # 更新失败也要让用户能继续用旧版本。
  Start-Process -FilePath $Target
  exit 1
} finally {
  Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue
}
`
}

func writeHelperScript(dir, name, body string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create helper directory: %w", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		return "", fmt.Errorf("write helper script: %w", err)
	}
	return path, nil
}
