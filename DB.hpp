#pragma once

#include <sqlite3.h>
#include <fmt/format.h>

#include <string>
#include <vector>


class DB {
public:
    explicit DB(const std::string &path);

    ~DB() { sqlite3_close(db_); }

    DB(const DB&)            = delete;
    DB& operator=(const DB&) = delete;

    struct location_t {
        int         id;
        std::string image_url;
        double      lat;
        double      lng;
    };

    void addLocation(const std::string& image_url, double lat, double lng);

    location_t randomLocation();

    location_t locationById(int id);

    // ── Sessions ───────────────────────────────

    // Create a new game session, returns session_id
    std::string newSession(int location_id);

    struct session_t {
        std::string id;
        int         location_id;
        bool        guessed;
    };

    session_t sessionById(const std::string& sid);

    void markGuessed(const std::string& sid);

    // ── Scores ─────────────────────────────────

    void saveScore(const std::string& player_name, int score, double dist_km);

    struct score_row_t {
        std::string player_name;
        int         score;
        double      dist_km;
        std::string ts;
    };

    std::vector<score_row_t> topScores(int limit = 10);

private:
    sqlite3* db_ = nullptr;

    // Run a statement, throw on error
    void exec(const std::string& sql);

    typedef std::vector<std::vector<std::string>> Rows;
    Rows query(const std::string& sql);

    // Naive SQL-escape (single quotes only — sufficient for SQLite)
    static std::string escape(const std::string& s);

    // Generate a random hex session ID
    static std::string generateId();

    void createTables();


};

