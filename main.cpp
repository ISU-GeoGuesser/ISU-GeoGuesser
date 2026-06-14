/*
 * Uniguessr Backend — Drogon C++17
 *
 * Dependencies:
 *   - drogon        (HTTP framework)
 *   - sqlite3       (database)
 *   - nlohmann/json (JSON, header-only)
 */

#include <drogon/drogon.h>
#include <sqlite3.h>
#include <nlohmann/json.hpp>
#include <fmt/format.h>

#include <cmath>
#include <string>
#include <vector>
#include <random>
#include <sstream>
#include <iomanip>
#include <stdexcept>
#include <chrono>

// using namespace drogon;
using json = nlohmann::json;

// ─────────────────────────────────────────────
//  Constants
// ─────────────────────────────────────────────

static const double EARTH_RADIUS_KM = 6371.0;
static const int    MAX_SCORE       = 5000;
static const double MAX_DIST_KM     = 20000.0; // half Earth circumference

// ─────────────────────────────────────────────
//  Haversine distance (km)
// ─────────────────────────────────────────────

double haversine(double lat1, double lon1, double lat2, double lon2)
{
    auto toRad = [](double d) { return d * M_PI / 180.0; };

    double dLat = toRad(lat2 - lat1);
    double dLon = toRad(lon2 - lon1);

    double a = std::sin(dLat / 2) * std::sin(dLat / 2)
             + std::cos(toRad(lat1)) * std::cos(toRad(lat2))
             * std::sin(dLon / 2) * std::sin(dLon / 2);

    return EARTH_RADIUS_KM * 2.0 * std::atan2(std::sqrt(a), std::sqrt(1.0 - a));
}

// Score: 5000 at 0 km, falls off exponentially, 0 at ~20 000 km
int calcScore(double distKm)
{
    if (distKm <= 0.0) return MAX_SCORE;
    double ratio = std::max(0.0, 1.0 - (distKm / MAX_DIST_KM));
    return static_cast<int>(MAX_SCORE * ratio * ratio);
}

// ─────────────────────────────────────────────
//  Database helper
// ─────────────────────────────────────────────

class DB {
public:
    explicit DB(const std::string& path)
    {
        if (sqlite3_open(path.c_str(), &db_) != SQLITE_OK)
            throw std::runtime_error("Cannot open database: " + path);
        exec("PRAGMA journal_mode=WAL;"); // better concurrency
        createTables();
        // seedLocations();
    }

    ~DB() { sqlite3_close(db_); }

    // Prevent copying
    DB(const DB&)            = delete;
    DB& operator=(const DB&) = delete;

    // ── Locations ──────────────────────────────

    struct Location {
        int         id;
        std::string image_url;
        double      lat;
        double      lng;
    };

    void addLocation(const std::string& image_url, double lat, double lng)
    {
        exec(fmt::format("INSERT INTO locations (image_url, lat, lng) VALUES ('{}', {}, {});",
             escape(image_url), lat, lng));
    }

    Location randomLocation()
    {
        auto rows = query("SELECT id, image_url, lat, lng "
                          "FROM locations ORDER BY RANDOM() LIMIT 1;");
        if (rows.empty()) throw std::runtime_error("No locations in database");
        auto& r = rows[0];
        return { std::stoi(r[0]), r[1],
                 std::stod(r[2]), std::stod(r[3]) };
    }

    Location locationById(int id)
    {
        auto rows = query(fmt::format("SELECT id, image_url, lat, lng FROM locations WHERE id = {};", id));
        if (rows.empty()) throw std::runtime_error("Location not found");
        auto& r = rows[0];
        return { std::stoi(r[0]), r[1],
                 std::stod(r[2]), std::stod(r[3]) };
    }

    // ── Sessions ───────────────────────────────

    // Create a new game session, returns session_id
    std::string newSession(int location_id)
    {
        std::string sid = generateId();
        exec(fmt::format("INSERT INTO sessions (id, location_id, guessed) VALUES ('{}', {}, 0);",
             escape(sid), location_id));
        return sid;
    }

    struct Session {
        std::string id;
        int         location_id;
        bool        guessed;
    };

    Session sessionById(const std::string& sid)
    {
        auto rows = query(fmt::format("SELECT id, location_id, guessed FROM sessions WHERE id = '{}';",
                                                 escape(sid)));
        if (rows.empty()) throw std::runtime_error("Session not found");
        auto& r = rows[0];
        return { r[0], std::stoi(r[1]), r[2] == "1" };
    }

    void markGuessed(const std::string& sid)
    {
        exec(fmt::format("UPDATE sessions SET guessed = 1 WHERE id = '{}';", escape(sid)));
    }

    // ── Scores ─────────────────────────────────

    void saveScore(const std::string& player_name, int score, double dist_km)
    {
        exec(fmt::format("INSERT INTO scores (player_name, score, dist_km, ts) VALUES ('{}', {}, {}, datetime('now'));", 
             escape(player_name), score, dist_km));
    }

    struct ScoreRow {
        std::string player_name;
        int         score;
        double      dist_km;
        std::string ts;
    };

    std::vector<ScoreRow> topScores(int limit = 10)
    {
        auto rows = query(fmt::format("SELECT player_name, score, dist_km, ts FROM scores ORDER BY score DESC LIMIT;", limit));
        std::vector<ScoreRow> out;
        for (auto& r : rows)
            out.push_back({ r[0], std::stoi(r[1]), std::stod(r[2]), r[3] });
        return out;
    }

private:
    sqlite3* db_ = nullptr;

    // Run a statement, throw on error
    void exec(const std::string& sql)
    {
        char* err = nullptr;
        if (sqlite3_exec(db_, sql.c_str(), nullptr, nullptr, &err) != SQLITE_OK) {
            std::string msg(err);
            sqlite3_free(err);
            throw std::runtime_error("SQL error: " + msg);
        }
    }

    // Run a SELECT, return rows as vector<vector<string>>
    using Rows = std::vector<std::vector<std::string>>;
    Rows query(const std::string& sql)
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

    // Naive SQL-escape (single quotes only — sufficient for SQLite)
    static std::string escape(const std::string& s)
    {
        std::string out;
        out.reserve(s.size());
        for (char c : s) {
            if (c == '\'') out += "''";
            else           out += c;
        }
        return out;
    }

    // Generate a random hex session ID
    static std::string generateId()
    {
        static std::mt19937_64 rng(std::random_device{}());
        std::ostringstream oss;
        oss << std::hex << std::setw(16) << std::setfill('0') << rng();
        return oss.str();
    }

    void createTables()
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


};

// ─────────────────────────────────────────────
//  CORS helper — add to every response
// ─────────────────────────────────────────────

drogon::HttpResponsePtr withCors(drogon::HttpResponsePtr resp)
{
    resp->addHeader("Access-Control-Allow-Origin",  "*");
    resp->addHeader("Access-Control-Allow-Methods", "GET, POST, OPTIONS");
    resp->addHeader("Access-Control-Allow-Headers", "Content-Type");
    return resp;
}

drogon::HttpResponsePtr jsonResp(const json& body, drogon::HttpStatusCode status = drogon::k200OK)
{
    auto resp = drogon::HttpResponse::newHttpResponse();
    resp->setStatusCode(status);
    resp->setContentTypeCode(drogon::CT_APPLICATION_JSON);
    resp->setBody(body.dump());
    return withCors(resp);
}

drogon::HttpResponsePtr errorResp(const std::string& msg, drogon::HttpStatusCode status = drogon::k400BadRequest)
{
    return jsonResp({ {"error", msg} }, status);
}

// ─────────────────────────────────────────────
//  main
// ─────────────────────────────────────────────

int main()
{
    auto db = std::make_shared<DB>("uniguessr.db");

    // ── OPTIONS preflight (CORS) ──────────────
    drogon::app().registerHandler(".*",
        [](const drogon::HttpRequestPtr&,
           std::function<void(const drogon::HttpResponsePtr&)>&& cb)
        {
            auto resp = drogon::HttpResponse::newHttpResponse();
            resp->setStatusCode(drogon::k204NoContent);
            cb(withCors(resp));
        },
        { drogon::Options }
    );

    // ── GET /api/round/new ────────────────────
    //    Returns a new session and the scene image URL.
    //    The actual coordinates are NOT sent to the client.
    drogon::app().registerHandler("/api/round/new",
        [db](const drogon::HttpRequestPtr&,
             std::function<void(const drogon::HttpResponsePtr&)>&& cb)
        {
            try {
                auto loc = db->randomLocation();
                auto sid = db->newSession(loc.id);

                cb(jsonResp({
                    { "session_id", sid        },
                    { "image_url",  loc.image_url },
                }));
            }
            catch (const std::exception& e) {
                cb(errorResp(e.what(), drogon::k500InternalServerError));
            }
        },
        { drogon::Get }
    );

    // ── POST /api/guess/{session_id} ──────────
    //    Body: { "lat": <float>, "lng": <float>, "player": "<name>" }
    //    Returns: { "score", "distance_km", "actual_lat", "actual_lng" }
    drogon::app().registerHandler("/api/guess/{session_id}",
        [db](const drogon::HttpRequestPtr& req,
             std::function<void(const drogon::HttpResponsePtr&)>&& cb,
             const std::string& session_id)
        {
            try {
                // Parse session
                auto session = db->sessionById(session_id);
                if (session.guessed) {
                    cb(errorResp("This round has already been guessed."));
                    return;
                }

                // Parse body
                auto bodyStr = req->getBody();
                if (bodyStr.empty()) {
                    cb(errorResp("Empty request body"));
                    return;
                }
                json body = json::parse(bodyStr);

                if (!body.contains("lat") || !body.contains("lng")) {
                    cb(errorResp("Body must contain 'lat' and 'lng'"));
                    return;
                }

                double guessLat    = body["lat"].get<double>();
                double guessLng    = body["lng"].get<double>();
                std::string player = body.value("player", "Anonymous");

                // Look up actual location
                auto loc = db->locationById(session.location_id);

                // Score
                double distKm = haversine(guessLat, guessLng, loc.lat, loc.lng);
                int    score  = calcScore(distKm);

                // Persist
                db->markGuessed(session_id);
                db->saveScore(player, score, distKm);

                cb(jsonResp({
                    { "score",       score          },
                    { "distance_km", distKm         },
                    { "actual_lat",  loc.lat        },
                    { "actual_lng",  loc.lng        },
                }));
            }
            catch (const json::exception& e) {
                cb(errorResp(std::string("JSON parse error: ") + e.what()));
            }
            catch (const std::exception& e) {
                cb(errorResp(e.what(), drogon::k500InternalServerError));
            }
        },
        { drogon::Post }
    );

    // ── GET /api/leaderboard ──────────────────
    //    Returns top 10 scores
    drogon::app().registerHandler("/api/leaderboard",
        [db](const drogon::HttpRequestPtr&,
             std::function<void(const drogon::HttpResponsePtr&)>&& cb)
        {
            try {
                auto rows = db->topScores(10);
                json arr = json::array();
                for (auto& r : rows) {
                    arr.push_back({
                        { "player",   r.player_name },
                        { "score",    r.score       },
                        { "dist_km",  r.dist_km     },
                        { "time",     r.ts          }
                    });
                }
                cb(jsonResp({ { "leaderboard", arr } }));
            }
            catch (const std::exception& e) {
                cb(errorResp(e.what(), drogon::k500InternalServerError));
            }
        },
        { drogon::Get }
    );

    // ── POST /api/admin/location ──────────────
    //    Multipart form: image (file), lat, lng
    //    Saves image to public/images/ and adds to database
    drogon::app().registerHandler("/api/admin/location",
        [db](const drogon::HttpRequestPtr& req,
            std::function<void(const drogon::HttpResponsePtr&)>&& cb)
        {
            try {
                auto contentType = req->getHeader("Content-Type");
                
                // Handle multipart form data (file upload)
                if (contentType.find("multipart/form-data") != std::string::npos) {
                    drogon::MultiPartParser parser;
                    if (parser.parse(req) != 0) {
                        cb(errorResp("Failed to parse multipart data"));
                        return;
                    }

                    auto& files = parser.getFiles();
                    auto& params = parser.getParameters();

                    if (files.empty() || !params.count("lat") || !params.count("lng")) {
                        cb(errorResp("Missing required fields: image, lat, lng"));
                        return;
                    }

                    // Get uploaded file
                    auto& file = files[0];
                    std::string originalName = file.getFileName();
                    
                    // Generate unique filename with timestamp
                    auto now = std::chrono::system_clock::now();
                    auto timestamp = std::chrono::duration_cast<std::chrono::milliseconds>(
                        now.time_since_epoch()).count();
                    
                    // Extract extension
                    std::string ext = ".jpg";
                    auto dotPos = originalName.find_last_of('.');
                    if (dotPos != std::string::npos) {
                        ext = originalName.substr(dotPos);
                    }
                    
                    std::string filename = "location_" + std::to_string(timestamp) + ext;
                    std::string filepath = "../public/images/" + filename;
                    std::string imageUrl = "/images/" + filename;

                    // Save file
                    file.saveAs(filepath);

                    // Parse coordinates
                    double lat = std::stod(params.at("lat"));
                    double lng = std::stod(params.at("lng"));

                    // Add to database
                    db->addLocation(imageUrl, lat, lng);

                    cb(jsonResp({
                        { "success", true },
                        { "message", "Location added successfully" },
                        { "image_url", imageUrl }
                    }));
                }
                // Handle JSON (for testing)
                else {
                    auto body = json::parse(req->getBody());
                    std::string img = body["image_url"];
                    double lat = body["lat"];
                    double lng = body["lng"];
                    
                    db->addLocation(img, lat, lng);
                    cb(jsonResp({
                        { "success", true },
                        { "message", "Location added" },
                        { "image_url", img }
                    }));
                }
            }
            catch (const std::exception& e) {
                cb(errorResp(e.what()));
            }
        },
        { drogon::Post }
    );

    // ── GET /admin ────────────────────────────
    //    Serve admin.html without needing .html extension
    drogon::app().registerHandler("/admin",
        [](const drogon::HttpRequestPtr&,
           std::function<void(const drogon::HttpResponsePtr&)>&& cb)
        {
            auto resp = drogon::HttpResponse::newFileResponse("../public/admin.html");
            cb(resp);
        },
        { drogon::Get }
    );

    // ── Serve static files (your HTML/CSS/JS) ─
    drogon::app().setDocumentRoot("../public");  // put index.html in ./public/

    drogon::app()
        .addListener("0.0.0.0", 8080)
        .setThreadNum(4)
        .run();

    return 0;
}