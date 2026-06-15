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

#include <cmath>

#include <chrono>

#include "DB.hpp"

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

int calcScore(double distKm)
{
    if (distKm <= 0.0) return MAX_SCORE;
    double ratio = std::max(0.0, 1.0 - (distKm / MAX_DIST_KM));
    return static_cast<int>(MAX_SCORE * ratio * ratio);
}

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
    auto db = std::make_shared<DB>("../uniguessr.db");

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