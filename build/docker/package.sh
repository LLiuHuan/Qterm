#!/usr/bin/env bash
# 在 Linux 容器里把一次构建打成「安装包 + 便携包」。
#
# 为什么在这个镜像里做：darwin 的 .app/.dmg 与 windows 的 .exe/NSIS 都能在这里交叉产出
# （zig + macOS SDK + makensis），dmg 用纯 Go 的 UDIF 写入器，不依赖 hdiutil。
# 因此一台机器（甚至一台 Mac）就能出全平台产物，不要求本机装 Windows/Linux。
#
# 输入（环境变量）：
#   APP_NAME   产物基名，默认 qterm（必须与 frontend/dist 里的应用一致）
#   TARGETS    逗号分隔，可选 darwin,windows,linux；默认全部
#   OUT        产物输出目录（容器内绝对路径），默认 /app/bin/artifacts
#   HOST_ARCH  linux 目标架构，默认取 `go env GOARCH`；linux 只能打容器自身架构
#   SKIP_DMG   非空时跳过 dmg（只出 .app 的 zip），用于快速迭代
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
export GIT_COMMITTER_NAME=${GIT_COMMITTER_NAME:-Qterm}
export GIT_COMMITTER_EMAIL=${GIT_COMMITTER_EMAIL:-qterm@users.noreply.github.com}

log() { printf '\n[package] %s\n' "$*"; }
has_target() { case ",${TARGETS}," in *",$1,"*) return 0 ;; *) return 1 ;; esac; }

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

package_darwin() {
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

    if [ -z "${SKIP_DMG}" ]; then
      log "生成 dmg darwin/${arch}"
      wails3 tool package --format dmg --name "${APP_NAME}" --out "${dir}" \
        --background "${REPO_ROOT}/build/darwin/dmg-background.png" \
        --volume-icon "${REPO_ROOT}/build/darwin/icons.icns" \
        --file-icon "${REPO_ROOT}/build/darwin/dmg-file-icon.icns" \
        --window-width 540 --window-height 380 >/dev/null
      mv "${dir}/${APP_NAME}.dmg" "${OUT}/qterm-darwin-${arch}.dmg"
    fi

    log "打包 darwin/${arch} 便携 zip"
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

    log "生成 NSIS 安装包 windows/${arch}"
    ( cd build/windows/nsis && makensis -DARG_WAILS_${flag}_BINARY="${dir}/${APP_NAME}.exe" project.nsi >/dev/null )
    mv "${REPO_ROOT}/bin/${APP_NAME}-${arch}-installer.exe" "${OUT}/qterm-windows-${arch}-setup.exe"

    log "打包 windows/${arch} 便携 zip"
    ( cd "${dir}" && zip -qry "${OUT}/qterm-windows-${arch}.zip" "${APP_NAME}.exe" )
  done
}

package_linux() {
  local arch dir
  arch="${HOST_ARCH}"
  dir=$(mktemp -d "/tmp/qterm-linux-${arch}.XXXXXX")
  go_build linux "${arch}" gcc "${dir}/${APP_NAME}"

  # nfpm 配置里写的是 ./bin/qterm，这里保持该布局，避免为打包改配置。
  mkdir -p bin && cp "${dir}/${APP_NAME}" "bin/${APP_NAME}"
  wails3 generate .desktop -name "${APP_NAME}" -exec "${APP_NAME}" -icon "${APP_NAME}" \
    -outputfile "${REPO_ROOT}/build/linux/${APP_NAME}.desktop" -categories "Development" >/dev/null

  log "生成 deb / rpm linux/${arch}"
  GOARCH="${arch}" wails3 tool package -name "${APP_NAME}" -format deb \
    -config ./build/linux/nfpm/nfpm.yaml -out "${OUT}" >/dev/null
  GOARCH="${arch}" wails3 tool package -name "${APP_NAME}" -format rpm \
    -config ./build/linux/nfpm/nfpm.yaml -out "${OUT}" >/dev/null

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
  mv "${OUT}/${APP_NAME}-${appimage_label}.AppImage" "${OUT}/qterm-linux-${arch}.AppImage"
  # 临时 builddir 留在容器 /tmp 里即可：容器用 --rm 启动，随进程销毁，不做递归删除。

  log "打包 linux/${arch} 便携 tar.gz"
  tar -czf "${OUT}/qterm-linux-${arch}.tar.gz" -C "${dir}" "${APP_NAME}"
}

has_target darwin && package_darwin
has_target windows && package_windows
has_target linux && package_linux

log "产物清单"
ls -lh "${OUT}" | sed 's/^/  /'
