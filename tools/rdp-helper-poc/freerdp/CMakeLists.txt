cmake_minimum_required(VERSION 3.20)
project(qterm_freerdp_helper C)

set(CMAKE_C_STANDARD 11)
set(CMAKE_C_STANDARD_REQUIRED ON)

# 链接方式：从"静态安装树"里 find_package，而不是直接链构建目录里的 .dylib。
#
# 为什么（2026-11-19 真实故障）：旧写法把 helper 链到构建机的动态库上，于是二进制里
# 留下 /opt/homebrew/opt/openssl@4/... 这类绝对路径。构建机升级 Homebrew、清掉旧 keg 后，
# 用户机器上 dyld 直接 abort —— 现场只有一行 "Library not loaded"，界面与日志都看不出原因。
# 现在 FreeRDP 以 -DBUILD_SHARED_LIBS=OFF 安装到 FREERDP_INSTALL_PREFIX，CMake 会把
# winpr/zlib/OpenSSL 等传递依赖自动带进来，产出的 helper 只剩系统库依赖，拷到任何机器都能跑。
# macOS/Linux 传入自建的静态安装前缀；Windows 由 vcpkg 工具链提供 FreeRDP，无需该变量。
if(FREERDP_INSTALL_PREFIX)
  list(PREPEND CMAKE_PREFIX_PATH "${FREERDP_INSTALL_PREFIX}")
endif()

# 包名随来源不同：自建的静态安装树导出 FreeRDPConfig.cmake，vcpkg 的 freerdp 端口把配置
# 挪到 share/freerdp 但保留上游文件名（新版上游可能带主版本号，如 FreeRDP3Config.cmake）。
# 各来源包名不一致，逐个回落，最后用 target 是否存在判定——2026-11-20 的 windows-amd64 /
# windows-arm64 job 就是只认 FreeRDP 时配置失败（包已装好，却报找不到 FreeRDPConfig.cmake）。
find_package(FreeRDP CONFIG QUIET)
if(NOT TARGET freerdp)
  find_package(FreeRDP3 CONFIG QUIET)
endif()
if(NOT TARGET freerdp)
  find_package(freerdp CONFIG QUIET)
endif()
if(NOT TARGET freerdp)
  find_package(freerdp3 CONFIG QUIET)
endif()
if(NOT TARGET freerdp)
  message(FATAL_ERROR
    "找不到 FreeRDP 的 CMake 包（尝试过 FreeRDP / FreeRDP3 / freerdp / freerdp3）。"
    "CMAKE_PREFIX_PATH=${CMAKE_PREFIX_PATH}；"
    "请用 -DFreeRDP3_DIR 指向含 <包名>Config.cmake 的目录。")
endif()

# 同理：FreeRDP-Client 侧的文件名可能是 FreeRDP-ClientConfig.cmake / FreeRDP-Client3Config.cmake，
# WinPR 侧可能是 WinPRConfig.cmake / WinPR3Config.cmake；导出的 target 名在所有情况下都一致
# （freerdp-client / winpr），所以链接语句不用改。
find_package(FreeRDP-Client CONFIG QUIET)
if(NOT TARGET freerdp-client)
  find_package(FreeRDP-Client3 CONFIG QUIET)
endif()
if(NOT TARGET freerdp-client)
  find_package(freerdp-client3 CONFIG QUIET)
endif()
if(NOT TARGET freerdp-client)
  message(FATAL_ERROR
    "找不到 FreeRDP-Client 的 CMake 包（尝试过 FreeRDP-Client / FreeRDP-Client3 / freerdp-client3）")
endif()

find_package(WinPR CONFIG QUIET)
if(NOT TARGET winpr)
  find_package(WinPR3 CONFIG QUIET)
endif()
if(NOT TARGET winpr)
  find_package(winpr3 CONFIG QUIET)
endif()
if(NOT TARGET winpr)
  message(FATAL_ERROR "找不到 WinPR 的 CMake 包（尝试过 WinPR / WinPR3 / winpr3）")
endif()

add_executable(qterm-freerdp-helper main.c)
target_link_libraries(qterm-freerdp-helper PRIVATE freerdp-client freerdp winpr)

# FreeRDP 3.9 安装出来的 CMake target 把 include 目录声明成 "${prefix}/include"，
# 但头文件实际落在 "${prefix}/include/freerdp3/freerdp/..."（pkg-config 也是给 -I.../include/freerdp3）。
# 只靠 target 的 INTERFACE_INCLUDE_DIRECTORIES 会 `fatal error: 'freerdp/channels/cliprdr.h'
# file not found`（2026-11-19 Linux rdp-helper 构建事故）；各 Config.cmake 另外导出了正确的
# *_INCLUDE_DIR 变量，这里显式补进目标。
foreach(_freerdp_inc_var FreeRDP-Client_INCLUDE_DIR FreeRDP_INCLUDE_DIR WinPR_INCLUDE_DIR)
  if(DEFINED ${_freerdp_inc_var} AND EXISTS "${${_freerdp_inc_var}}")
    target_include_directories(qterm-freerdp-helper PRIVATE "${${_freerdp_inc_var}}")
  endif()
endforeach()

# 静态链接后没有运行期库查找，但保留安装规则，方便 CI 直接取产物。
install(TARGETS qterm-freerdp-helper RUNTIME DESTINATION bin)
