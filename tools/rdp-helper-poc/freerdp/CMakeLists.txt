cmake_minimum_required(VERSION 3.20)
project(qterm_freerdp_helper C)

set(CMAKE_C_STANDARD 11)
set(CMAKE_C_STANDARD_REQUIRED ON)

if(NOT FREERDP_SOURCE_DIR OR NOT FREERDP_BUILD_DIR)
  message(FATAL_ERROR "请指定 FREERDP_SOURCE_DIR 与 FREERDP_BUILD_DIR")
endif()

add_executable(qterm-freerdp-helper main.c)
target_include_directories(qterm-freerdp-helper PRIVATE
  "${FREERDP_SOURCE_DIR}/include"
  "${FREERDP_SOURCE_DIR}/winpr/include"
  "${FREERDP_BUILD_DIR}/include"
  "${FREERDP_BUILD_DIR}/winpr/include"
)
target_link_libraries(qterm-freerdp-helper PRIVATE
  "${FREERDP_BUILD_DIR}/client/common/libfreerdp-client3.dylib"
  "${FREERDP_BUILD_DIR}/libfreerdp/libfreerdp3.dylib"
  "${FREERDP_BUILD_DIR}/winpr/libwinpr/libwinpr3.dylib"
)
set_target_properties(qterm-freerdp-helper PROPERTIES
  BUILD_RPATH "${FREERDP_BUILD_DIR}/client/common;${FREERDP_BUILD_DIR}/libfreerdp;${FREERDP_BUILD_DIR}/winpr/libwinpr"
)
