cmake_minimum_required(VERSION 3.15)
project(uniguessr CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_OSX_DEPLOYMENT_TARGET "11.0" CACHE STRING "Minimum macOS deployment version")
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)

# ── Dependencies ──────────────────────────────────────────────────────────────

find_package(Drogon REQUIRED)

# nlohmann/json — install via:  sudo apt install nlohmann-json3-dev
#             or:               vcpkg install nlohmann-json
find_package(nlohmann_json REQUIRED)

# Do NOT call find_package(SQLite3) — Drogon's own FindSQLite3.cmake already
# creates a target called SQLite3_lib and a second find_package would conflict.
# Instead, tell the linker to link the library directly by name.
# Homebrew installs it at /opt/homebrew/opt/sqlite/lib/libsqlite3.dylib on Apple Silicon.
find_library(SQLITE3_LIB
    NAMES sqlite3
    HINTS /opt/homebrew/opt/sqlite/lib   # Apple Silicon Homebrew
          /usr/local/opt/sqlite/lib       # Intel Homebrew
          /usr/lib
          /usr/local/lib
    REQUIRED
)

# fmt — install via: brew install fmt
find_library(FMT_LIB
    NAMES fmt
    HINTS /opt/homebrew/opt/fmt/lib   # Apple Silicon Homebrew
          /usr/local/opt/fmt/lib       # Intel Homebrew
          /usr/lib
          /usr/local/lib
    REQUIRED
)

# ── Target ────────────────────────────────────────────────────────────────────

add_executable(uniguessr main.cpp)

target_include_directories(uniguessr PRIVATE
    /opt/homebrew/opt/sqlite/include    # Apple Silicon
    /usr/local/opt/sqlite/include       # Intel Mac
    /opt/homebrew/opt/fmt/include       # fmt library
    /usr/local/opt/fmt/include
)

target_link_libraries(uniguessr
    PRIVATE
        Drogon::Drogon
        nlohmann_json::nlohmann_json
        ${FMT_LIB}       # fmt library
        ${SQLITE3_LIB}   # raw path — bypasses the target-name conflict entirely
)

# ── Optional: copy public/ folder next to the binary ─────────────────────────

add_custom_command(TARGET uniguessr POST_BUILD
    COMMAND ${CMAKE_COMMAND} -E copy_directory
        ${CMAKE_SOURCE_DIR}/public
        $<TARGET_FILE_DIR:uniguessr>/public
    COMMENT "Copying public/ to build directory"
)