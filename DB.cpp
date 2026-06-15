#include "DB.hpp"

#include <random>
#include <sstream>
#include <iomanip>
#include <stdexcept>

DB::DB(const std::string &path)
{
    if (sqlite3_open(path.c_str(), &db_) != SQLITE_OK)
        throw std::runtime_error("Cannot open database: " + path);
    exec("PRAGMA journal_mode=WAL;");
    createTables();
}


void DB::addLocation(const std::string& image_url, double lat, double lng)
{
    exec(fmt::format("INSERT INTO locations (image_url, lat, lng) VALUES ('{}', {}, {});",
            escape(image_url), lat, lng));
}

DB::location_t DB::randomLocation()
{
    auto rows = query("SELECT id, image_url, lat, lng "
                        "FROM locations ORDER BY RANDOM() LIMIT 1;");
    if (rows.empty()) throw std::runtime_error("No locations in database");
    auto& r = rows[0];
    return { std::stoi(r[0]), r[1],
                std::stod(r[2]), std::stod(r[3]) };
}

DB::location_t DB::locationById(int id)
{
    auto rows = query(fmt::format("SELECT id, image_url, lat, lng FROM locations WHERE id = {};", id));
    if (rows.empty()) throw std::runtime_error("Location not found");
    auto& r = rows[0];
    return { std::stoi(r[0]), r[1],
                std::stod(r[2]), std::stod(r[3]) };
}

    // ── Sessions ───────────────────────────────

    // Create a new game session, returns session_id
std::string DB::newSession(int location_id)
{
    std::string sid = generateId();
    exec(fmt::format("INSERT INTO sessions (id, location_id, guessed) VALUES ('{}', {}, 0);",
            escape(sid), location_id));
    return sid;
}

DB::session_t DB::sessionById(const std::string& sid)
{
    auto rows = query(fmt::format("SELECT id, location_id, guessed FROM sessions WHERE id = '{}';",
                                                escape(sid)));
    if (rows.empty()) throw std::runtime_error("Session not found");
    auto& r = rows[0];
    return { r[0], std::stoi(r[1]), r[2] == "1" };
}

void DB::markGuessed(const std::string& sid)
{
    exec(fmt::format("UPDATE sessions SET guessed = 1 WHERE id = '{}';", escape(sid)));
}

void DB::saveScore(const std::string& player_name, int score, double dist_km)
{
    exec(fmt::format("INSERT INTO scores (player_name, score, dist_km, ts) VALUES ('{}', {}, {}, datetime('now'));", 
            escape(player_name), score, dist_km));
}

std::vector<DB::score_row_t> DB::topScores(int limit)
{
    auto rows = query(fmt::format("SELECT player_name, score, dist_km, ts FROM scores ORDER BY score DESC LIMIT {};", limit));
    std::vector<score_row_t> out;
    for (auto& r : rows)
        out.push_back({ r[0], std::stoi(r[1]), std::stod(r[2]), r[3] });
    return out;
}

void DB::exec(const std::string& sql)
{
    char* err = nullptr;
    if (sqlite3_exec(db_, sql.c_str(), nullptr, nullptr, &err) != SQLITE_OK) {
        std::string msg(err);
        sqlite3_free(err);
        throw std::runtime_error("SQL error: " + msg);
    }
}

// using Rows = std::vector<std::vector<std::string>>;
DB::Rows DB::query(const std::string& sql)
{
    Rows result;
    sqlite3_stmt* stmt = nullptr;
    if (sqlite3_prepare_v2(db_, sql.c_str(), -1, &stmt, nullptr) != SQLITE_OK)
        throw std::runtime_error("Prepare failed: " + sql);

    while (sqlite3_step(stmt) == SQLITE_ROW) {
        std::vector<std::string> row;
        int cols = sqlite3_column_count(stmt);
        for (int i = 0; i < cols; ++i) {
            const char* val = reinterpret_cast<const char*>(sqlite3_column_text(stmt, i));
            row.emplace_back(val ? val : "");
        }
        result.push_back(std::move(row));
    }
    sqlite3_finalize(stmt);
    return result;
}

std::string DB::escape(const std::string& s)
{
    std::string out;
    out.reserve(s.size());
    for (char c : s) {
        if (c == '\'') out += "''";
        else           out += c;
    }
    return out;
}

std::string DB::generateId()
{
    static std::mt19937_64 rng(std::random_device{}());
    std::ostringstream oss;
    oss << std::hex << std::setw(16) << std::setfill('0') << rng();
    return oss.str();
}

void DB::createTables()
{
    exec(R"(
        CREATE TABLE IF NOT EXISTS locations (
            id        INTEGER PRIMARY KEY AUTOINCREMENT,
            image_url TEXT NOT NULL,
            lat       REAL NOT NULL,
            lng       REAL NOT NULL
        );

        CREATE TABLE IF NOT EXISTS sessions (
            id          TEXT PRIMARY KEY,
            location_id INTEGER NOT NULL,
            guessed     INTEGER NOT NULL DEFAULT 0,
            FOREIGN KEY (location_id) REFERENCES locations(id)
        );

        CREATE TABLE IF NOT EXISTS scores (
            id          INTEGER PRIMARY KEY AUTOINCREMENT,
            player_name TEXT NOT NULL,
            score       INTEGER NOT NULL,
            dist_km     REAL NOT NULL,
            ts          TEXT NOT NULL
        );
    )");
}

