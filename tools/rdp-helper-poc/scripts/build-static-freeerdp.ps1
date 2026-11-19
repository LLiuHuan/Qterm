# 构建静态链接的 FreeRDP helper（Windows / MSVC + vcpkg）。
#
# 为什么走 vcpkg：FreeRDP 官方只保证 MSVC 工具链，MinGW 交叉编出来的产物在真实
# Windows 上问题很多；vcpkg 的 `x64-windows-static` 三元组会同时把 OpenSSL、zlib、libusb
# 都编成静态库，正好满足「安装包只带一个 exe」的目标。
#
# 用法（在 Developer PowerShell 里或 CI 的 windows runner 上）：
#   pwsh -File tools/rdp-helper-poc/scripts/build-static-freeerdp.ps1
#   pwsh -File ... -VcpkgRoot C:\vcpkg -Triplet x64-windows-static
#
# 产物：tools/rdp-helper-poc/freerdp/build/Release/qterm-freerdp-helper.exe
# 注意：本脚本尚未在真实 Windows 上验证过，首次上 CI 时请先看 rdp-helper workflow 的日志。
param(
  [string]$VcpkgRoot = $env:VCPKG_INSTALLATION_ROOT,
  [string]$Triplet = 'x64-windows-static'
)

$ErrorActionPreference = 'Stop'
$Root = (Resolve-Path (Join-Path $PSScriptRoot '../../..')).Path

if (-not $VcpkgRoot) {
  throw '找不到 vcpkg：请设置 VCPKG_INSTALLATION_ROOT，或用 -VcpkgRoot 指定'
}

$Vcpkg = Join-Path $VcpkgRoot 'vcpkg.exe'
if (-not (Test-Path $Vcpkg)) {
  throw "vcpkg.exe 不存在：$Vcpkg"
}

Write-Host "==> vcpkg install freerdp:$Triplet（首次会编译全部依赖，约 10-20 分钟）"
& $Vcpkg install "freerdp:$Triplet"
if ($LASTEXITCODE -ne 0) { throw "vcpkg install 失败：$LASTEXITCODE" }

$HelperBuild = Join-Path $Root 'tools/rdp-helper-poc/freerdp/build'
$Toolchain = Join-Path $VcpkgRoot 'scripts/buildsystems/vcpkg.cmake'

Write-Host "==> 配置 helper"
& cmake -S (Join-Path $Root 'tools/rdp-helper-poc/freerdp') -B $HelperBuild `
  -DCMAKE_BUILD_TYPE=Release `
  -DCMAKE_TOOLCHAIN_FILE="$Toolchain" `
  -DVCPKG_TARGET_TRIPLET=$Triplet
if ($LASTEXITCODE -ne 0) { throw "cmake 配置失败：$LASTEXITCODE" }

Write-Host "==> 构建 helper"
& cmake --build $HelperBuild --config Release --parallel
if ($LASTEXITCODE -ne 0) { throw "cmake 构建失败：$LASTEXITCODE" }

$Helper = Join-Path $HelperBuild 'Release/qterm-freerdp-helper.exe'
if (-not (Test-Path $Helper)) { throw "未找到产物：$Helper" }

Write-Host "==> 检查动态依赖（只应出现系统 DLL）"
& dumpbin /dependents $Helper
Write-Host "OK: $Helper"
