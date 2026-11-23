#!/usr/bin/env bash
# 构建静态链接的 FreeRDP helper（macOS / Linux）。
#
# 为什么要有这个脚本：静态化涉及一长串很容易漏的 CMake 参数，漏一个就会把构建机的
# /opt/homebrew/... 绝对路径重新写进二进制，用户机器上 dyld 直接 abort（2026-11-19 事故）。
# CI 与本地都只走这一条路径，保证"开发机能跑"与"发布包能跑"是同一套参数。
#
# 用法：
#   tools/rdp-helper-poc/scripts/build-static-freeerdp.sh
#   FREERDP_SRC=/path/to/freerdp JOBS=4 tools/rdp-helper-poc/scripts/build-static-freeerdp.sh
#
# 产物：tools/rdp-helper-poc/freerdp/build/qterm-freerdp-helper
# 前置：FreeRDP 源码在 ${FREERDP_SRC}（默认 .cache/freerdp-src），且系统有 OpenSSL 静态库。
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
FREERDP_SRC="${FREERDP_SRC:-${ROOT_DIR}/.cache/freerdp-src}"
FREERDP_BUILD="${FREERDP_BUILD:-${ROOT_DIR}/.cache/freerdp-build-static}"
FREERDP_INSTALL="${FREERDP_INSTALL:-${ROOT_DIR}/.cache/freerdp-install-static}"
HELPER_BUILD="${ROOT_DIR}/tools/rdp-helper-poc/freerdp/build"
JOBS="${JOBS:-$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 4)}"

if [ ! -d "${FREERDP_SRC}" ]; then
  echo "找不到 FreeRDP 源码：${FREERDP_SRC}" >&2
  echo "先取源码，例如：git clone --depth 1 --branch 3.9.0 https://github.com/FreeRDP/FreeRDP.git ${FREERDP_SRC}" >&2
  exit 1
fi

# OpenSSL 静态库的位置按平台确定。
case "$(uname -s)" in
  Darwin)
    # Apple Silicon 的 Homebrew 在 /opt/homebrew，Intel 的（macos-15-intel runner）在 /usr/local；
    # 写死 /opt/homebrew 会让 Intel 上找不到 libssl.a（2026-11-19 darwin-amd64 job 事故）。
    if [ -z "${OPENSSL_PREFIX:-}" ] && command -v brew >/dev/null 2>&1; then
      OPENSSL_PREFIX="$(brew --prefix openssl@3 2>/dev/null || true)"
    fi
    if [ -z "${OPENSSL_PREFIX:-}" ]; then
      for candidate in /opt/homebrew/opt/openssl@3 /usr/local/opt/openssl@3; do
        if [ -d "${candidate}" ]; then
          OPENSSL_PREFIX="${candidate}"
          break
        fi
      done
    fi
    OPENSSL_PREFIX="${OPENSSL_PREFIX:-/opt/homebrew/opt/openssl@3}"
    ;;
  Linux)
    OPENSSL_PREFIX="${OPENSSL_PREFIX:-$(pkg-config --variable=prefix openssl 2>/dev/null || echo /usr)}"
    ;;
  *)
    echo "当前脚本只支持 macOS 与 Linux；Windows 用 build-static-freeerdp.ps1" >&2
    exit 1
    ;;
esac

# 必须**显式**传 .a 路径：CMake 缓存里一旦有 .dylib 值，OPENSSL_USE_STATIC_LIBS 不会覆盖它，
# 导出的目标就会把动态库路径带进 helper 的链接接口。
OPENSSL_SSL_LIB="$(ls "${OPENSSL_PREFIX}"/lib/libssl.a "${OPENSSL_PREFIX}"/lib/*/libssl.a 2>/dev/null | head -n1 || true)"
OPENSSL_CRYPTO_LIB="$(ls "${OPENSSL_PREFIX}"/lib/libcrypto.a "${OPENSSL_PREFIX}"/lib/*/libcrypto.a 2>/dev/null | head -n1 || true)"
if [ -z "${OPENSSL_SSL_LIB}" ] || [ -z "${OPENSSL_CRYPTO_LIB}" ]; then
  echo "在 ${OPENSSL_PREFIX} 下找不到 libssl.a / libcrypto.a（headless 环境通常要装 OpenSSL 静态库）" >&2
  echo "Debian/Ubuntu: apt-get install -y libssl-dev；Alpine: apk add openssl-dev openssl-libs-static" >&2
  exit 1
fi

# 只保留 helper 真正需要的库：libfreerdp / libfreerdp-client / libwinpr。
# SDL 客户端、server、sample 一律关掉，否则会额外拖进 C++ filesystem、wayland 等无关依赖；
# FFmpeg/swscale 也必须关，否则 configure 阶段就要求构建机装有 ffmpeg 开发包。
CMAKE_ARGS=(
  -DCMAKE_BUILD_TYPE=Release
  -DBUILD_SHARED_LIBS=OFF
  -DOPENSSL_ROOT_DIR="${OPENSSL_PREFIX}"
  -DOPENSSL_INCLUDE_DIR="${OPENSSL_PREFIX}/include"
  -DOPENSSL_USE_STATIC_LIBS=ON
  -DOPENSSL_SSL_LIBRARY="${OPENSSL_SSL_LIB}"
  -DOPENSSL_CRYPTO_LIBRARY="${OPENSSL_CRYPTO_LIB}"
  -DCHANNEL_URBDRC=OFF
  -DCHANNEL_URBDRC_CLIENT=OFF
  -DWITH_X11=OFF
  -DWITH_CAIRO=OFF
  -DWITH_CUPS=OFF
  -DWITH_WAYLAND=OFF
  -DWITH_FFMPEG=OFF
  -DWITH_DSP_FFMPEG=OFF
  -DWITH_VIDEO_FFMPEG=OFF
  -DWITH_SWSCALE=OFF
  -DWITH_CLIENT=OFF
  -DWITH_SERVER=OFF
  -DWITH_SAMPLE=OFF
  -DWITH_SERVER_INTERFACE=OFF
  -DWITH_SERVER_CHANNELS=OFF
  -DWITH_MANPAGES=OFF
  # 关掉 winpr 的全部 JSON 后端。JSON 只服务 Azure AD（WITH_AAD）和「从 JSON 文件读
  # 时区/键盘布局」这两个默认关闭的特性，helper 一个都不用；不关的话，构建机上只要有
  # json-c 的 CMake 包（macOS runner 自带），CMake 就认定 json-c 可用并把
  # winpr/libwinpr/utils/json/json-c.c 编进来，而 include 目录没跟着传，直接
  # `fatal error: 'json-c/json.h' file not found`（2026-11-19 rdp-helper CI 事故）。
  -DWITH_JSON_DISABLED=ON
  # 关掉 Kerberos。FreeRDP 在「非 Windows、非 Apple」平台把 WITH_KRB5 默认设为 ON，
  # 于是 configure 阶段强制 find_package(KRB5 REQUIRED)；CI 的 ubuntu runner 没装
  # libkrb5-dev，直接 `Could NOT find Kerberos`（2026-11-19 rdp-helper CI 事故）。
  # 就算装上了，静态链接还得额外提供 libkrb5.a 等一串库，helper 也用不到 Kerberos 认证。
  -DWITH_KRB5=OFF
  # NTLM 的 MD4/RC4 必须用 winpr 自带实现，不能指望 OpenSSL 的 legacy provider。
  # macOS（Homebrew）和 Ubuntu 的 libcrypto.a 都是带 module 编的：legacy provider 是独立
  # 的 legacy.so/.dylib，静态链接时进不了二进制，于是连接 NLA（CredSSP）主机时日志里只剩
  #   winpr_openssl_initialize: OpenSSL LEGACY provider failed to load, no md4 support available!
  #   winpr_Digest_Init_Internal: Failed to initialize digest md4
  #   credssp_auth_authenticate: InitializeSecurityContext failed with SEC_E_INTERNAL_ERROR
  # 结果就是能连到 TLS、却在 NLA 阶段被打回（2026-11-23 用户在真实 Windows 主机上撞到）。
  # FreeRDP 自己打 deb/Windows/macOS 包时也是开这两个内部实现的。
  -DWITH_INTERNAL_MD4=ON
  -DWITH_INTERNAL_RC4=ON
  # Linux 上 winpr 的 CRT 层默认 find_package(ICU REQUIRED ...)，而 ICU 只提供动态库
  # 的发行版（或没装 libicu-dev 的 runner）会直接 configure 失败；helper 只需要
  # UTF-16/UTF-8 互转，用 winpr 自带的 unicode_builtin.c 就够（FreeRDP 自己的
  # oss-fuzz/coverity 预置也是这么开的）。
  -DWITH_UNICODE_BUILTIN=ON
  # FUSE 只用于「剪贴板文件复制」走 fuse3 挂载，Linux 上默认 ON 且 pkg_check_modules
  # 是 REQUIRED；runner 没装 libfuse3-dev 时 configure 直接失败。helper 不做剪贴板文件传输。
  -DWITH_FUSE=OFF
  -DCMAKE_INSTALL_PREFIX="${FREERDP_INSTALL}"
)

echo "==> 配置 FreeRDP 静态构建（${FREERDP_BUILD}）"
cmake -S "${FREERDP_SRC}" -B "${FREERDP_BUILD}" "${CMAKE_ARGS[@]}"

echo "==> 构建 FreeRDP（-j${JOBS}）"
cmake --build "${FREERDP_BUILD}" --parallel "${JOBS}"
cmake --install "${FREERDP_BUILD}" >/dev/null

echo "==> 构建 helper（${HELPER_BUILD}）"
cmake -S "${ROOT_DIR}/tools/rdp-helper-poc/freerdp" -B "${HELPER_BUILD}" \
  -DCMAKE_BUILD_TYPE=Release \
  -DFREERDP_INSTALL_PREFIX="${FREERDP_INSTALL}"
cmake --build "${HELPER_BUILD}" --parallel "${JOBS}"

HELPER="${HELPER_BUILD}/qterm-freerdp-helper"
echo "==> 检查动态依赖（除系统库外应为空）"
EXTERNAL=""
case "$(uname -s)" in
  Darwin)
    EXTERNAL="$(otool -L "${HELPER}" | tail -n +2 | awk '{print $1}' | grep -vE '^(/System/|/usr/lib/)' || true)"
    ;;
  Linux)
    if ! ldd "${HELPER}" 2>&1 | grep -q 'not a dynamic executable'; then
      EXTERNAL="$(ldd "${HELPER}" | awk '{print $3}' | grep -v '^$' | grep -vE '^(/lib/|/usr/lib/)' || true)"
    fi
    ;;
esac
if [ -n "${EXTERNAL}" ]; then
  echo "helper 仍依赖非系统库，静态化失败：" >&2
  echo "${EXTERNAL}" >&2
  exit 1
fi
echo "OK: ${HELPER}"
