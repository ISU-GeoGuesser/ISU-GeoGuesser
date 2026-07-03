# ISU-GeoGuesser
A geoguesser website for the Illinois State University campus.

## Running the MVP

Requires Go 1.26+. From the repo root:

```sh
go build -C backend -o ../reggieguessr .
./reggieguessr
```

Then open http://localhost:8080 — it redirects to the home page, and the
Single Player / Multiplayer buttons create a game and drop you into
`/game/`. The server must run from the repo root so it can serve the
`home/` and `game/` folders.

Tests:

```sh
cd backend && go test .
```

## What the MVP includes

- **Gameplay page** (`game/`) — full round loop: campus photo, click the
  Leaflet map to drop a pin, lock in the guess, post-round reveal with
  everyone's pins + the real location, next round, final standings.
- **Round timer** — 60s per round by default; guesses lock automatically
  when it expires. `GET /games/start?rounds=5&seconds=60` configures both
  per game (rounds 1–10, seconds 5–180).
- **Multiplayer** — every game is a room; share the lobby link and the
  round resolves when all connected players have guessed. Single Player
  is the same game with `&solo=1`, which skips the lobby.
- **Hardcoded locations** (`backend/locations.go`) — 8 campus spots with
  approximate real coordinates so scoring can be tested. The photos are
  placeholders; swap this slice for database rows once the photo library
  and admin upload flow exist.

## Scoring

Same model as GeoGuessr, scaled from the Earth down to campus
(`backend/scoring.go`):

```
score = 5000 * exp(-10 * distance / mapDiagonal)
```

GeoGuessr uses the world map's diagonal (~14,917 km); we use the ISU
campus diagonal (~2.5 km), so the curve feels identical: a perfect pin
(≤10 m) is 5000, a tenth of the map off (250 m) is ~1839, across campus
is ~0. Distance is haversine (great-circle) meters between the guess and
the photo's GPS coordinate.

## WebSocket protocol

Connect to `GET /games/:id` (websocket). Messages are
`{"t": "TYPE", "d": {...}}`.

Client → server: `HELLO {name}` · `START` · `GUESS {lat, lng}` · `NEXT`

Server → client: `JOINED` (you, room code, settings, players) ·
`PLAYERS` (roster + who has guessed) · `ROUND_START` (image, round,
endsAtMs) · `ROUND_RESULT` (actual location + per-player distance/score) ·
`GAME_OVER` (standings) · `ERROR`
