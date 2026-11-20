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

find_package(FreeRDP CONFIG REQUIRED)
# vcpkg 的 freerdp 端口会把 config 文件改名成带主版本号的小写形式
# （share/freerdp-client3/freerdp-client3-config.cmake、share/winpr3/winpr3-config.cmake），
# 而 find_package(FreeRDP-Client) 只找 FreeRDP-ClientConfig.cmake / freerdp-client-config.cmake，
# 于是 Windows 上直接 "Could NOT find FreeRDP-Client"。两种命名都试一遍；导出的 target 名
# （freerdp-client / winpr）两边一致，链接语句不用改。
find_package(FreeRDP-Client CONFIG QUIET)
if(NOT FreeRDP-Client_FOUND)
  find_package(freerdp-client3 CONFIG REQUIRED)
endif()
find_package(WinPR CONFIG QUIET)
if(NOT WinPR_FOUND)
  find_package(winpr3 CONFIG REQUIRED)
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
