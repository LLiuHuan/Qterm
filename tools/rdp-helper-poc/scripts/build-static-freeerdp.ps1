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

Write-Host "==> vcpkg install freerdp[client]:$Triplet（首次会编译全部依赖，约 10-30 分钟）"
# 必须带 [client] 特性：vcpkg 的 freerdp 默认不编客户端组件（WITH_CLIENT=OFF），
# 不带这个特性时安装树里没有 freerdp-client 库，helper 的 find_package(FreeRDP-Client)
# 会直接失败（2026-11-19 rdp-helper CI 事故）。
& $Vcpkg install "freerdp[client]:$Triplet"
if ($LASTEXITCODE -ne 0) { throw "vcpkg install 失败：$LASTEXITCODE" }

$HelperBuild = Join-Path $Root 'tools/rdp-helper-poc/freerdp/build'
$Toolchain = Join-Path $VcpkgRoot 'scripts/buildsystems/vcpkg.cmake'
# 三元组前缀决定生成器的目标平台：VS 生成器不接受“自动”平台，不显式传 -A 时
# arm64 三元组会被配进 x64 平台，链接阶段报架构不匹配。
$Platform = if ($Triplet -like 'arm64*') { 'ARM64' } else { 'x64' }

# 生成器名字跟着镜像里的 VS 版本走：windows-2022 是 VS 17（2022），而 windows-11-arm
# 自 2026-09 起换成 VS 18（2026）镜像。写死 'Visual Studio 17 2022' 会在新镜像上直接
# 报 "could not find any instance of Visual Studio"，job 连配置阶段都进不去。
function Get-VsGenerator {
  $vswhere = Join-Path ${env:ProgramFiles(x86)} 'Microsoft Visual Studio\Installer\vswhere.exe'
  if (-not (Test-Path $vswhere)) { return 'Visual Studio 17 2022' }
  $version = & $vswhere -latest -products * -property catalog_productDisplayVersion 2>$null
  if ($version -match '^18\.') { return 'Visual Studio 18 2026' }
  return 'Visual Studio 17 2022'
}

$Generator = Get-VsGenerator
Write-Host "==> 配置 helper（生成器 $Generator / 平台 $Platform）"
& cmake -S (Join-Path $Root 'tools/rdp-helper-poc/freerdp') -B $HelperBuild `
  -G $Generator -A $Platform `
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
if (Get-Command dumpbin -ErrorAction SilentlyContinue) {
  & dumpbin /dependents $Helper
} else {
  # dumpbin 只在 VS 开发者环境里可用；静态三元组本身已保证没有 FreeRDP/OpenSSL 运行时依赖。
  Write-Warning '找不到 dumpbin，跳过依赖检查'
}
Write-Host "OK: $Helper"
