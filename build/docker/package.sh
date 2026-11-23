#!/usr/bin/env bash
# 在 Linux 容器里把一次构建打成「安装包 + 便携包」。
#
# 为什么在这个镜像里做：darwin 的 .app 中间包与 windows 的 .exe/NSIS 都能在这里交叉产出
# （zig + macOS SDK + makensis）；macOS 最终 ZIP/DMG 交给 macOS 主机签名与封装。
# 因此一台机器（甚至一台 Mac）就能出全平台产物，不要求本机装 Windows/Linux。
#
# 输入（环境变量）：
#   APP_NAME   产物基名，默认 qterm（必须与 frontend/dist 里的应用一致）
#   TARGETS    逗号分隔，可选 darwin,windows,linux；默认全部
#   OUT        产物输出目录（容器内绝对路径），默认 /app/bin/artifacts
#   HOST_ARCH  linux 目标架构，默认取 `go env GOARCH`；linux 只能打容器自身架构
#   DEFER_DARWIN_SIGNING 必须为 1：容器里的 macOS 包只能作为待签名中间制品
#   SKIP_DMG   macOS 中间阶段必须非空；最终 DMG 由 macOS 主机在签名后生成
#
# 前置条件：`frontend/dist` 必须已构建（前端在宿主机跑 npm，容器只编译 Go）。
set -euo pipefail

APP_NAME=${APP_NAME:-qterm}
TARGETS=${TARGETS:-darwin,windows,linux}
OUT=${OUT:-/app/bin/artifacts}
HOST_ARCH=${HOST_ARCH:-$(go env GOARCH)}
SKIP_DMG=${SKIP_DMG:-}
REPO_ROOT=$(pwd)

# nfpm 的 maintainer 直接读这两个变量；不设会打出 `Maintainer:  <>` 的空元数据。
export GIT_COMMITTER_NAME=${GIT_COMMITTER_NAME:-LLiuHuan}
export GIT_COMMITTER_EMAIL=${GIT_COMMITTER_EMAIL:-qterm@users.noreply.github.com}

log() { printf '\n[package] %s\n' "$*"; }
has_target() { case ",${TARGETS}," in *",$1,"*) return 0 ;; *) return 1 ;; esac; }

# RDP helper 由 rdp-helper workflow 在各自平台原生编出来（Linux 的 glibc 静态、Windows 的
# MSVC/vcpkg 都没法从 macOS 交叉产出），release.yml 会把它落到按平台分目录的
# tools/rdp-helper-poc/freerdp/build/<goos>-<goarch>/ 下。
#
# 为什么按架构分目录：一次发版要出 amd64 与 arm64 两套包，而 helper 是**目标架构**的
# 原生二进制。放同一个扁平路径会让后下载的覆盖先下载的（2026-11-19 事故：arm64 helper
# 被塞进 amd64 的包，用户机器上直接 exec format error）。
#
# 各打包步骤（nfpm 的 render-config.mjs、darwin 的 Taskfile）读的是扁平路径，所以这里
# 把当前正在打包的架构的 helper 拷过去；没有对应架构的 helper 就返回非 0，调用方按
# "不带 RDP"处理。
RDP_HELPER_DIR="${REPO_ROOT}/tools/rdp-helper-poc/freerdp/build"
stage_rdp_helper() {
  local goos=$1 goarch=$2 name="qterm-freerdp-helper" src dst
  [ "${goos}" = "windows" ] && name="${name}.exe"
  src="${RDP_HELPER_DIR}/${goos}-${goarch}/${name}"
  dst="${RDP_HELPER_DIR}/${name}"
  rm -f "${dst}"
  if [ ! -f "${src}" ]; then
    return 1
  fi
  # 这里必须是真实文件而不是软链：nfpm 会把软链原样打进 deb/rpm（包内留下一个指向
  # linux-arm64/… 的断链，用户机器上 RDP 直接不可用）；AppImage 重打包与 Taskfile 的
  # `-f` 判断也会因跨容器/宿主机的路径解析失败而静默跳过。
  cp -f "${src}" "${dst}"
  chmod 0755 "${dst}"
  return 0
}

if [ ! -d "${REPO_ROOT}/frontend/dist" ]; then
  echo "[package] 缺少 frontend/dist：请先在宿主机执行 npm --prefix frontend run build" >&2
  exit 1
fi

mkdir -p "${OUT}" "${OUT}/raw"

# go_build <os> <arch> <cc> <output> <extra-ldflags>
go_build() {
  local goos=$1 goarch=$2 cc=$3 output=$4 extra=${5:-}
  log "编译 ${goos}/${goarch}"
  local cgo=1 flags="-w -s${extra}"
  if [ "${goos}" = "windows" ]; then cgo=0; fi
  env CGO_ENABLED="${cgo}" GOOS="${goos}" GOARCH="${goarch}" CC="${cc}" \
    go build -tags production -trimpath -buildvcs=false -ldflags "${flags}" -o "${output}" .
}

# go_build_cli <os> <arch> <output>
#
# 命令行客户端（qterm-cli）与主程序的两点差异，都不能靠复用 go_build 抹平：
#   1. 它是纯 Go 实现，固定 CGO_ENABLED=0，不需要 CC（容器里那套 zig 包装是给主程序用的）；
#   2. LDFLAGS 不能带 -H windowsgui —— 那是 GUI 子系统开关，加到 CLI 上会让它在 Windows
#      上没有控制台输出（stdout/stderr 全丢，等于不可用）。
# 二进制名是产品常量 qterm-cli，不跟 APP_NAME 走：Go 侧 binaryName 写死，改名会找不到。
go_build_cli() {
  local goos=$1 goarch=$2 output=$3
  log "编译命令行客户端 ${goos}/${goarch}"
  env CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" \
    go build -tags production -trimpath -buildvcs=false -ldflags "-w -s" -o "${output}" ./cmd/qterm
}

package_darwin() {
  # 不能用 linker 对裸二进制的签名充当完整 .app 签名。
  if [ "${DEFER_DARWIN_SIGNING:-}" != "1" ]; then
    echo "[package] macOS 包必须交给 macOS 完成最终签名；只允许 DEFER_DARWIN_SIGNING=1 的中间构建" >&2
    exit 1
  fi
  if [ -z "${SKIP_DMG}" ]; then
    echo "[package] macOS DMG 必须在完成签名后由 macOS 最终封装" >&2
    exit 1
  fi
  local arch dir app
  for arch in amd64 arm64; do
    # 每次用全新的临时目录：不在挂载进来的仓库里写中间产物，也就不需要递归删除。
    dir=$(mktemp -d "/tmp/qterm-darwin-${arch}.XXXXXX")
    go_build darwin "${arch}" "zcc-darwin-${arch}" "${dir}/${APP_NAME}"

    app="${dir}/${APP_NAME}.app"
    mkdir -p "${app}/Contents/MacOS" "${app}/Contents/Resources"
    cp build/darwin/icons.icns "${app}/Contents/Resources/"
    cp "${dir}/${APP_NAME}" "${app}/Contents/MacOS/"
    cp build/darwin/Info.plist "${app}/Contents/"

    # RDP helper 只有对当前架构编过才带（Contents/Resources/helpers/，Go 侧按该目录查找）。
    # 缺失时包照常出，只是界面提示"未安装 RDP helper"。
    if stage_rdp_helper darwin "${arch}"; then
      mkdir -p "${app}/Contents/Resources/helpers"
      cp "${RDP_HELPER_DIR}/qterm-freerdp-helper" "${app}/Contents/Resources/helpers/"
      chmod 0755 "${app}/Contents/Resources/helpers/qterm-freerdp-helper"
    else
      log "darwin/${arch} 没有 RDP helper，本次包不带 RDP"
    fi

    # 命令行客户端：放进 Contents/Resources/bin/，与 build/darwin/Taskfile.yml 里
    # create:app:bundle 的布局一致；Go 侧 binaryCandidates() 按
    # 「可执行文件同目录的 ../Resources/bin/qterm-cli」查找。
    mkdir -p "${app}/Contents/Resources/bin"
    go_build_cli darwin "${arch}" "${app}/Contents/Resources/bin/qterm-cli"
    chmod 0755 "${app}/Contents/Resources/bin/qterm-cli"


    log "打包 darwin/${arch} 待签名中间 zip（不可直接发布）"
    ( cd "${dir}" && zip -qry "${OUT}/qterm-darwin-${arch}.zip" "${APP_NAME}.app" )
  done
}

package_windows() {
  local arch dir flag syso
  for arch in amd64 arm64; do
    dir=$(mktemp -d "/tmp/qterm-windows-${arch}.XXXXXX")
    flag=$([ "${arch}" = "amd64" ] && echo AMD64 || echo ARM64)

    # 图标与清单由 .syso 嵌进 exe；用完必须删掉，否则会污染其它架构的构建。
    syso="wails_windows_${arch}.syso"
    wails3 generate syso -arch "${arch}" -icon build/windows/icon.ico \
      -manifest build/windows/wails.exe.manifest -info build/windows/info.json -out "${syso}" >/dev/null
    go_build windows "${arch}" "zcc-windows-${arch}" "${dir}/${APP_NAME}.exe" " -H windowsgui"
    rm -f "${syso}"

    # RDP helper：project.nsi 的 File 是相对 build/windows/nsis 的固定文件名，先拷进去、
    # 打完立刻删掉，避免把二进制留在工作区；便携 zip 直接放进临时目录一起压。
    local helper_ok=0
    if stage_rdp_helper windows "${arch}"; then
      helper_ok=1
      cp "${RDP_HELPER_DIR}/qterm-freerdp-helper.exe" build/windows/nsis/qterm-freerdp-helper.exe
      cp "${RDP_HELPER_DIR}/qterm-freerdp-helper.exe" "${dir}/"
    else
      log "windows/${arch} 没有 RDP helper，本次包不带 RDP"
    fi

    # 命令行客户端：project.nsi 的 File 同样相对 build/windows/nsis，先拷进去、
    # 打完立刻删掉，避免把二进制留在工作区；便携 zip 直接放进临时目录一起压。
    # 装完之后 qterm-cli.exe 与 qterm.exe 同目录，Go 侧按「可执行文件同目录」查找。
    go_build_cli windows "${arch}" "${dir}/qterm-cli.exe"
    cp "${dir}/qterm-cli.exe" build/windows/nsis/qterm-cli.exe

    log "生成 NSIS 安装包 windows/${arch}"
    # 必须显式指定 per-user 安装范围：project.nsi 的默认值是 machine，
    # 会装进 C:\Program Files，而应用清单是 asInvoker、普通权限运行，
    # 装好之后连自动更新都写不进去（探针建不出临时文件，报 target-readonly）。
    # per-user 装到 %LOCALAPPDATA%\Programs\...，当前用户始终可写，不需要 UAC。
    ( cd build/windows/nsis && makensis \
      -DWAILS_INSTALL_SCOPE=user -DREQUEST_EXECUTION_LEVEL=user \
      -DARG_WAILS_${flag}_BINARY="${dir}/${APP_NAME}.exe" project.nsi >/dev/null )
    mv "${REPO_ROOT}/bin/${APP_NAME}-${arch}-installer.exe" "${OUT}/qterm-windows-${arch}-setup.exe"
    if [ "${helper_ok}" = "1" ]; then
      rm -f build/windows/nsis/qterm-freerdp-helper.exe
    fi
    rm -f build/windows/nsis/qterm-cli.exe

    log "打包 windows/${arch} 便携 zip"
    local zip_files=("${APP_NAME}.exe" "qterm-cli.exe")
    if [ "${helper_ok}" = "1" ]; then
      zip_files+=("qterm-freerdp-helper.exe")
    fi
    ( cd "${dir}" && zip -qry "${OUT}/qterm-windows-${arch}.zip" "${zip_files[@]}" )
  done
}

package_linux() {
  local arch dir tar_files
  arch="${HOST_ARCH}"
  dir=$(mktemp -d "/tmp/qterm-linux-${arch}.XXXXXX")
  go_build linux "${arch}" gcc "${dir}/${APP_NAME}"

  # 先把当前架构的 helper 拷到扁平路径：下面的 nfpm 配置与 AppImage 重打包都读这个固定路径。
  if stage_rdp_helper linux "${arch}"; then
    log "linux/${arch} 带上 RDP helper"
  else
    log "linux/${arch} 没有 RDP helper，本次包不带 RDP"
  fi

  # nfpm 配置里写的是 ./bin/qterm，这里保持该布局，避免为打包改配置。
  mkdir -p bin && cp "${dir}/${APP_NAME}" "bin/${APP_NAME}"
  # 命令行客户端：nfpm 配置读 ./bin/qterm-cli，AppImage 重打包与便携包也读同一份。
  go_build_cli linux "${arch}" "bin/qterm-cli"
  chmod 0755 "bin/qterm-cli"
  wails3 generate .desktop -name "${APP_NAME}" -exec "${APP_NAME}" -icon "${APP_NAME}" \
    -outputfile "${REPO_ROOT}/build/linux/${APP_NAME}.desktop" -categories "Development" >/dev/null

  # RDP helper 只有对平台编过才存在：这里生成"存在则打包"的 nfpm 配置。
  node ./build/linux/nfpm/render-config.mjs

  log "生成 deb / rpm linux/${arch}"
  GOARCH="${arch}" wails3 tool package -name "${APP_NAME}" -format deb \
    -config ./build/linux/nfpm/nfpm.generated.yaml -out "${OUT}" >/dev/null
  GOARCH="${arch}" wails3 tool package -name "${APP_NAME}" -format rpm \
    -config ./build/linux/nfpm/nfpm.generated.yaml -out "${OUT}" >/dev/null

  # wails3 tool package 的输出名固定是 <name>.deb / <name>.rpm（不带版本与架构），
  # 这里统一改成发行物命名，避免两个架构互相覆盖。
  mv "${OUT}/${APP_NAME}.deb" "${OUT}/qterm-linux-${arch}.deb"
  mv "${OUT}/${APP_NAME}.rpm" "${OUT}/qterm-linux-${arch}.rpm"

  log "生成 AppImage linux/${arch}"
  # linuxdeploy 在容器里跑需要 libfuse2；image 里已装，且允许解包执行（无 /dev/fuse 也能跑）。
  # builddir 必须每次是空目录：复用上一次的 AppDir 会让 linuxdeploy 在「打包」阶段失败（实测）。
  local appimage_build appimage_label
  appimage_build=$(mktemp -d /tmp/qterm-appimage.XXXXXX)
  appimage_label=$([ "${arch}" = "amd64" ] && echo x86_64 || echo aarch64)
  rm -f "${OUT}/${APP_NAME}-${appimage_label}.AppImage"
  wails3 generate appimage -binary "bin/${APP_NAME}" -icon frontend/public/app-icons/linux.png \
    -desktopfile "build/linux/${APP_NAME}.desktop" -outputdir "${OUT}" \
    -builddir "${appimage_build}" >/dev/null

  # wails3 的 appimage 命令没有"附加文件"参数（只把 -binary 拷进 AppDir/usr/bin），
  # 所以把 helper 放进 AppDir/usr/bin 后，用同一次下载的 linuxdeploy 重新打一次包。
  # AppRun 已经把 usr/bin 放进 PATH，Go 侧的 exec.LookPath 就能找到 helper。
  local appdir="${appimage_build}/${APP_NAME}-${appimage_label}.AppDir"
  local linuxdeploy="${appimage_build}/linuxdeploy-${appimage_label}.AppImage"
  local repacked=0
  if [ -x "${linuxdeploy}" ]; then
    # 命令行客户端放进 usr/bin/：命令名是 qterm-cli，与 GUI 主程序（usr/bin/qterm）
    # 不同名，不会互相覆盖；AppRun 把 usr/bin 加进 PATH 后直接就能敲。
    # Go 侧 binaryCandidates() 的「可执行文件同目录 qterm-cli」也正好命中。
    mkdir -p "${appdir}/usr/bin"
    cp bin/qterm-cli "${appdir}/usr/bin/qterm-cli"
    chmod 0755 "${appdir}/usr/bin/qterm-cli"
    if [ -f "${RDP_HELPER_DIR}/qterm-freerdp-helper" ]; then
      cp "${RDP_HELPER_DIR}/qterm-freerdp-helper" "${appdir}/usr/bin/qterm-freerdp-helper"
      chmod 0755 "${appdir}/usr/bin/qterm-freerdp-helper"
    fi
    # NO_STRIP：新版工具链的 .relr.dyn 段会让 linuxdeploy 自带的 strip 失败。
    ( cd "${appimage_build}" && OUTPUT="${APP_NAME}-${appimage_label}.AppImage" NO_STRIP=1 \
        "${linuxdeploy}" --appimage-extract-and-run --appdir "${appdir}" --output appimage >/dev/null )
    mv -f "${appimage_build}/${APP_NAME}-${appimage_label}.AppImage" "${OUT}/qterm-linux-${arch}.AppImage"
    # wails3 打包前已把未带 helper 的那份 AppImage 移到 OUT，重打包成功后要删掉它，
    # 否则发版脚本会把它当成同名产物上传。
    rm -f "${OUT}/${APP_NAME}-${appimage_label}.AppImage"
    repacked=1
  else
    # 拿不到 linuxdeploy 就没法重打包，此时 AppImage 里没有命令行客户端。
    log "linux/${arch} 缺少 linuxdeploy，AppImage 不带命令行客户端"
  fi
  if [ "${repacked}" = "0" ]; then
    mv "${OUT}/${APP_NAME}-${appimage_label}.AppImage" "${OUT}/qterm-linux-${arch}.AppImage"
  fi
  # 临时 builddir 留在容器 /tmp 里即可：容器用 --rm 启动，随进程销毁，不做递归删除。

  log "打包 linux/${arch} 便携 tar.gz"
  # 便携包把 helper 放在主程序同目录：Go 侧 helperBinaryPath 的最后一条查找规则就是
  # "可执行文件同目录"，不依赖 PATH 或安装目录。命令行客户端同理。
  tar_files=("${APP_NAME}" "qterm-cli")
  cp bin/qterm-cli "${dir}/qterm-cli"
  chmod 0755 "${dir}/qterm-cli"
  if [ -f "${RDP_HELPER_DIR}/qterm-freerdp-helper" ]; then
    cp "${RDP_HELPER_DIR}/qterm-freerdp-helper" "${dir}/qterm-freerdp-helper"
    chmod 0755 "${dir}/qterm-freerdp-helper"
    tar_files+=("qterm-freerdp-helper")
  fi
  tar -czf "${OUT}/qterm-linux-${arch}.tar.gz" -C "${dir}" "${tar_files[@]}"
}

has_target darwin && package_darwin
has_target windows && package_windows
has_target linux && package_linux

log "产物清单"
ls -lh "${OUT}" | sed 's/^/  /'
