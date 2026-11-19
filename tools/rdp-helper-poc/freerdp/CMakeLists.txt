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
find_package(FreeRDP-Client CONFIG REQUIRED)
find_package(WinPR CONFIG REQUIRED)

add_executable(qterm-freerdp-helper main.c)
target_link_libraries(qterm-freerdp-helper PRIVATE freerdp-client freerdp winpr)

# 静态链接后没有运行期库查找，但保留安装规则，方便 CI 直接取产物。
install(TARGETS qterm-freerdp-helper RUNTIME DESTINATION bin)
