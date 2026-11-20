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

# vcpkg 的 freerdp 端口用 vcpkg_cmake_config_fixup 把配置从 lib/cmake/<目录> 挪到
# share/<包名>，但**不会改配置文件名**：FreeRDP 3.x 装出来的是 FreeRDPConfig.cmake /
# FreeRDP-ClientConfig.cmake / WinPRConfig.cmake（新版可能带主版本号，如
# FreeRDP3Config.cmake）。而 find_package(FreeRDP) 只认 FreeRDPConfig.cmake 与
# freerdp-config.cmake 两种名字，一旦上游改名就必然报「Could not find a package
# configuration file provided by "FreeRDP"」——2026-11-20 的 windows-amd64 /
# windows-arm64 job 就是这么红的（包明明装好了）。
# 这里先按文件名把配置目录找出来，再给所有可能的包名都塞 <包名>_DIR，让 CMakeLists 里
# 的名字回落链能命中；找不到时把安装树打出来，避免下一轮还得靠猜。
$Installed = Join-Path $VcpkgRoot "installed/$Triplet"
if (-not (Test-Path $Installed)) {
  throw "vcpkg 安装树不存在：$Installed（VCPKG_INSTALLATION_ROOT=$VcpkgRoot）"
}

function Resolve-PackageDir {
  param([scriptblock]$Filter)
  foreach ($base in @((Join-Path $Installed 'share'), (Join-Path $Installed 'lib/cmake'))) {
    if (-not (Test-Path $base)) { continue }
    $hit = Get-ChildItem -Path $base -Recurse -Filter '*Config.cmake' -File -ErrorAction SilentlyContinue |
      Where-Object $Filter | Select-Object -First 1
    if ($hit) { return $hit.DirectoryName }
  }
  return $null
}

$CoreDir = Resolve-PackageDir {
  $_.Name -match 'reerdp' -and $_.Name -notmatch 'Client|Server|Proxy|Shadow|tools'
}
$ClientDir = Resolve-PackageDir { $_.Name -match 'reerdp' -and $_.Name -match 'Client' }
$WinprDir = Resolve-PackageDir { $_.Name -match 'winpr' -and $_.Name -notmatch 'tools' }

if (-not $CoreDir) {
  Write-Host "==> 安装树：$Installed"
  Write-Host '==> share 下的目录：'
  Get-ChildItem -Path (Join-Path $Installed 'share') -Directory -ErrorAction SilentlyContinue |
    ForEach-Object { Write-Host "    $($_.Name)" }
  Write-Host '==> lib/cmake 下的目录：'
  Get-ChildItem -Path (Join-Path $Installed 'lib/cmake') -Directory -ErrorAction SilentlyContinue |
    ForEach-Object { Write-Host "    $($_.Name)" }
  throw "未找到 FreeRDP 的 CMake 配置文件（$Installed）"
}
Write-Host "==> FreeRDP 配置：$CoreDir"
if ($ClientDir) { Write-Host "==> FreeRDP-Client 配置：$ClientDir" }
if ($WinprDir) { Write-Host "==> WinPR 配置：$WinprDir" }

# 包名大小写/版本号各来源不一致，全部给上；CMakeLists 里按 target 是否存在逐个回落。
$DirArgs = @()
if ($CoreDir) {
  $DirArgs += "-DFreeRDP_DIR=$CoreDir", "-DFreeRDP3_DIR=$CoreDir",
    "-Dfreerdp_DIR=$CoreDir", "-Dfreerdp3_DIR=$CoreDir"
}
if ($ClientDir) {
  $DirArgs += "-DFreeRDP-Client_DIR=$ClientDir", "-DFreeRDP-Client3_DIR=$ClientDir",
    "-Dfreerdp-client3_DIR=$ClientDir"
}
if ($WinprDir) {
  $DirArgs += "-DWinPR_DIR=$WinprDir", "-DWinPR3_DIR=$WinprDir", "-Dwinpr3_DIR=$WinprDir"
}

Write-Host "==> 配置 helper（生成器 $Generator / 平台 $Platform）"
Write-Host "==> vcpkg 安装树：$Installed"
Write-Host "==> 包目录参数：$($DirArgs -join ' ')"
# vcpkg 工具链优先读环境变量，其次是 -DVCPKG_TARGET_TRIPLET；两条都给，避免某一侧被
# PowerShell 的引号/续行吃掉后静默退化成默认三元组（x64-windows 动态库）。
$env:VCPKG_DEFAULT_TRIPLET = $Triplet
& cmake -S (Join-Path $Root 'tools/rdp-helper-poc/freerdp') -B $HelperBuild `
  -G $Generator -A $Platform `
  -DCMAKE_BUILD_TYPE=Release `
  -DCMAKE_TOOLCHAIN_FILE="$Toolchain" `
  "-DVCPKG_TARGET_TRIPLET=$Triplet" `
  "-DVCPKG_INSTALLED_DIR=$(Join-Path $VcpkgRoot 'installed')" `
  "-DCMAKE_PREFIX_PATH=$Installed" `
  @DirArgs
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
