cmake_minimum_required(VERSION 3.15)
project(uniguessr CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# Only set macOS deployment target on Apple systems
if(APPLE)
    # Let CMake use the current system version to match homebrew libraries
    if(NOT CMAKE_OSX_DEPLOYMENT_TARGET)
        set(CMAKE_OSX_DEPLOYMENT_TARGET "11.0" CACHE STRING "Minimum macOS deployment version")
    endif()
endif()

set(CMAKE_EXPORT_COMPILE_COMMANDS ON)

# ── Dependencies ──────────────────────────────────────────────────────────────

# Disable install for vendored dependencies by overriding install() command
function(install)
endfunction()

add_subdirectory(vendor/json)
add_subdirectory(vendor/fmt)

# yaml-cpp - disable tests/tools for faster build
set(YAML_CPP_BUILD_TESTS OFF CACHE BOOL "" FORCE)
set(YAML_CPP_BUILD_TOOLS OFF CACHE BOOL "" FORCE)
set(YAML_CPP_BUILD_CONTRIB OFF CACHE BOOL "" FORCE)
add_subdirectory(vendor/yaml-cpp)

# Drogon - disable examples/tests for faster build
set(BUILD_EXAMPLES OFF CACHE BOOL "" FORCE)
set(BUILD_CTL OFF CACHE BOOL "" FORCE)
set(BUILD_TESTING OFF CACHE BOOL "" FORCE)
set(BUILD_DROGON_SHARED OFF CACHE BOOL "" FORCE)
add_subdirectory(vendor/drogon)

# Re-enable install() for our own project
unset(install)
macro(install)
    _install(${ARGV})
endmacro()

# ── SQLite3 ──
find_library(SQLITE3_LIB
    NAMES sqlite3 sqlite3.dll
    HINTS 
        /opt/homebrew/opt/sqlite/lib /usr/local/opt/sqlite/lib
        /usr/lib /usr/local/lib
        /usr/lib/x86_64-linux-gnu /usr/lib/aarch64-linux-gnu
        ${CMAKE_PREFIX_PATH}/lib
    PATH_SUFFIXES lib lib64
    REQUIRED
)

find_path(SQLITE3_INCLUDE_DIR
    NAMES sqlite3.h
    HINTS
        /opt/homebrew/opt/sqlite/include /usr/local/opt/sqlite/include
        /usr/include /usr/local/include
        ${CMAKE_PREFIX_PATH}/include
    REQUIRED
)

# ── Target ────────────────────────────────────────────────────────────────────

add_executable(uniguessr main.cpp DB.cpp)

# Include directories
target_include_directories(uniguessr PRIVATE ${SQLITE3_INCLUDE_DIR})
if(NOT fmt_FOUND AND NOT FMT_VENDORED)
    target_include_directories(uniguessr PRIVATE ${FMT_INCLUDE_DIR})
endif()

# Link libraries
target_link_libraries(uniguessr PRIVATE nlohmann_json::nlohmann_json)
target_link_libraries(uniguessr PRIVATE drogon)
target_link_libraries(uniguessr PRIVATE fmt::fmt)
target_link_libraries(uniguessr PRIVATE ${SQLITE3_LIB})

# ── Optional: copy public/ folder next to the binary ─────────────────────────

add_custom_command(TARGET uniguessr POST_BUILD
    COMMAND ${CMAKE_COMMAND} -E copy_directory
        ${CMAKE_SOURCE_DIR}/public
        $<TARGET_FILE_DIR:uniguessr>/public
    COMMENT "Copying public/ to build directory"
)