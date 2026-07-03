/* ReggieGuessr gameplay client.
 *
 * Talks to the Go backend over the team's websocket protocol
 * ({t: "TYPE", d: {...}} messages). One page handles the whole game:
 * lobby -> guessing -> reveal -> next round -> final results.
 */

"use strict";

// ---------- campus map constants ----------

// Center of the ISU Quad; bounds pad the campus so players can't wander
// off to Chicago. Tighter than GeoGuessr's world map on purpose.
const CAMPUS_CENTER = [40.5095, -88.992];
const CAMPUS_BOUNDS = L.latLngBounds([40.496, -89.012], [40.523, -88.972]);
const MAX_SCORE = 5000;

// ---------- page elements ----------

const els = {
    photo: document.getElementById("round-photo"),
    hudRound: document.getElementById("hud-round"),
    hudTimer: document.getElementById("hud-timer"),
    hudTimerBox: document.getElementById("hud-timer-box"),
    hudScore: document.getElementById("hud-score"),
    roster: document.getElementById("roster"),
    guessBtn: document.getElementById("guess-btn"),
    revealPanel: document.getElementById("reveal-panel"),
    revealTitle: document.getElementById("reveal-title"),
    revealLocation: document.getElementById("reveal-location"),
    revealResults: document.getElementById("reveal-results"),
    nextBtn: document.getElementById("next-btn"),
    lobby: document.getElementById("lobby"),
    lobbyPlayers: document.getElementById("lobby-players"),
    shareLink: document.getElementById("share-link"),
    copyBtn: document.getElementById("copy-btn"),
    nameInput: document.getElementById("name-input"),
    startBtn: document.getElementById("start-btn"),
    gameover: document.getElementById("gameover"),
    finalHeadline: document.getElementById("final-headline"),
    standings: document.getElementById("standings"),
    disconnected: document.getElementById("disconnected"),
    disconnectedReason: document.getElementById("disconnected-reason"),
    rejoinBtn: document.getElementById("rejoin-btn"),
    againBtn: document.getElementById("again-btn"),
    toast: document.getElementById("toast"),
};

const params = new URLSearchParams(location.search);
const soloMode = params.get("solo") === "1";

// ---------- state ----------

let ws = null;
let myName = localStorage.getItem("rg-name") || "";
let pendingGuess = null; // LatLng picked on the map, not yet submitted
let hasGuessed = false;
let totalScore = 0;
let roundEndsAtMs = 0;
let timerInterval = null;
let gameEnded = false;

function setState(state) {
    document.body.dataset.state = state;
    els.lobby.hidden = state !== "lobby";
    els.gameover.hidden = state !== "over";
    els.disconnected.hidden = state !== "disconnected";
    els.revealPanel.hidden = state !== "reveal";
    if (state === "reveal" || state === "guessing") {
        // The dock changes size between these states; Leaflet needs a nudge
        // to redraw tiles for the new box once the CSS transition settles.
        setTimeout(() => map.invalidateSize(), 350);
    }
}

function toast(message, ms = 2600) {
    els.toast.textContent = message;
    els.toast.hidden = false;
    clearTimeout(toast._t);
    toast._t = setTimeout(() => { els.toast.hidden = true; }, ms);
}

// ---------- Leaflet map ----------

const map = L.map("map", {
    center: CAMPUS_CENTER,
    zoom: 16,
    minZoom: 15,
    maxZoom: 19,
    maxBounds: CAMPUS_BOUNDS,
    maxBoundsViscosity: 1.0,
    zoomControl: false,
});

L.control.zoom({ position: "topleft" }).addTo(map);

L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
}).addTo(map);

// Pins are inline SVG so they can use brand colors without image assets.
function pinIcon(color) {
    return L.divIcon({
        className: "pin",
        html: `<svg viewBox="0 0 24 24" width="34" height="34">
            <path d="M12 1.5c-4.4 0-8 3.5-8 7.9 0 5.6 7.1 13.1 7.4 13.4a.8.8 0 0 0 1.2 0c.3-.3 7.4-7.8 7.4-13.4 0-4.4-3.6-7.9-8-7.9z" fill="${color}"/>
            <circle cx="12" cy="9.4" r="3" fill="#fff"/>
        </svg>`,
        iconSize: [34, 34],
        iconAnchor: [17, 32],
        tooltipAnchor: [0, -30],
    });
}

const MY_PIN = pinIcon("#ce0e2d");
const OTHER_PIN = pinIcon("#3a3a3a");
const ACTUAL_PIN = pinIcon("#1c7c3c");

let myMarker = null;
let revealLayer = L.layerGroup().addTo(map);

map.on("click", (e) => {
    if (document.body.dataset.state !== "guessing" || hasGuessed) return;
    pendingGuess = e.latlng;
    if (!myMarker) {
        myMarker = L.marker(pendingGuess, { icon: MY_PIN }).addTo(map);
    } else {
        myMarker.setLatLng(pendingGuess);
    }
    els.guessBtn.disabled = false;
    els.guessBtn.textContent = "Lock in this guess";
});

// ---------- round timer ----------

function renderTimer() {
    const remaining = Math.max(0, roundEndsAtMs - Date.now());
    const totalSec = Math.ceil(remaining / 1000);
    const m = Math.floor(totalSec / 60);
    const s = String(totalSec % 60).padStart(2, "0");
    els.hudTimer.textContent = `${m}:${s}`;
    els.hudTimerBox.classList.toggle("is-low", remaining > 0 && remaining <= 10_000);
    if (remaining <= 0) clearInterval(timerInterval);
}

function startTimer(endsAtMs) {
    roundEndsAtMs = endsAtMs;
    clearInterval(timerInterval);
    renderTimer();
    timerInterval = setInterval(renderTimer, 250);
}

// ---------- websocket protocol ----------

function send(type, data) {
    if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ t: type, d: data ?? null }));
    }
}

const handlers = {
    JOINED(d) {
        els.shareLink.value = location.origin + location.pathname + "?id=" + params.get("id");
        if (myName) send("HELLO", { name: myName });

        if (soloMode) {
            // Single player: no lobby, straight into round 1.
            if (!d.inProgress) send("START");
        } else if (!d.inProgress) {
            setState("lobby");
        }
        // If the game is already running, ROUND_START follows immediately.
    },

    PLAYERS(players) {
        renderRoster(players);
        renderLobbyPlayers(players);
    },

    ROUND_START(d) {
        hasGuessed = false;
        pendingGuess = null;
        revealLayer.clearLayers();
        if (myMarker) { map.removeLayer(myMarker); myMarker = null; }

        els.photo.src = d.image;
        els.photo.hidden = false;
        els.hudRound.textContent = `${d.round} / ${d.rounds}`;
        els.guessBtn.disabled = true;
        els.guessBtn.textContent = "Place your pin on the map";

        map.setView(CAMPUS_CENTER, 16);
        startTimer(d.endsAtMs);
        setState("guessing");
    },

    ROUND_RESULT(d) {
        clearInterval(timerInterval);
        els.hudTimer.textContent = "0:00";
        els.hudTimerBox.classList.remove("is-low");

        drawReveal(d);
        renderResults(d);

        els.nextBtn.textContent = d.round >= d.rounds ? "See final results" : "Next round";
        setState("reveal");
    },

    GAME_OVER(d) {
        gameEnded = true;
        renderStandings(d.standings);
        setState("over");
    },

    ERROR(d) {
        toast(d.message || "Something went wrong");
    },
};

function connect() {
    const id = params.get("id");
    const proto = location.protocol === "https:" ? "wss" : "ws";
    ws = new WebSocket(`${proto}://${location.host}/games/${id}`);

    ws.onmessage = (event) => {
        const msg = JSON.parse(event.data);
        const handler = handlers[msg.t];
        if (handler) handler(msg.d);
    };

    ws.onclose = () => {
        clearInterval(timerInterval);
        if (gameEnded) return; // closing after the final screen is fine
        els.disconnectedReason.textContent =
            "The game server closed the connection. Games idle out after a while.";
        setState("disconnected");
    };
}

// ---------- rendering ----------

function renderRoster(players) {
    els.roster.replaceChildren(...players.map((p) => {
        const li = document.createElement("li");
        li.className = "roster__player" + (p.guessed ? " has-guessed" : "");
        const dot = document.createElement("span");
        dot.className = "roster__dot";
        li.append(dot, document.createTextNode(p.name));
        return li;
    }));
}

function renderLobbyPlayers(players) {
    els.lobbyPlayers.replaceChildren(...players.map((p) => {
        const li = document.createElement("li");
        li.textContent = p.name;
        return li;
    }));
}

function drawReveal(d) {
    revealLayer.clearLayers();
    if (myMarker) { map.removeLayer(myMarker); myMarker = null; }

    const actual = L.latLng(d.actual.lat, d.actual.lng);
    L.marker(actual, { icon: ACTUAL_PIN, zIndexOffset: 1000 })
        .bindTooltip(d.actual.name, { className: "pin-label", permanent: true, direction: "top" })
        .addTo(revealLayer);

    const points = [actual];
    for (const r of d.results) {
        if (!r.guessed) continue;
        const guessPoint = L.latLng(r.lat, r.lng);
        points.push(guessPoint);
        L.marker(guessPoint, { icon: r.name === myName || d.results.length === 1 ? MY_PIN : OTHER_PIN })
            .bindTooltip(r.name, { className: "pin-label", direction: "top" })
            .addTo(revealLayer);
        L.polyline([guessPoint, actual], {
            color: "#ffffff", weight: 2, dashArray: "6 8", opacity: 0.8,
        }).addTo(revealLayer);
    }

    map.fitBounds(L.latLngBounds(points).pad(0.35), { maxZoom: 17 });
}

function formatDistance(meters) {
    if (meters < 1000) return `${Math.round(meters)} m`;
    return `${(meters / 1000).toFixed(2)} km`;
}

function renderResults(d) {
    els.revealTitle.textContent = `Round ${d.round} results`;
    els.revealLocation.innerHTML = "";
    els.revealLocation.append("This was ");
    const strong = document.createElement("strong");
    strong.textContent = d.actual.name;
    els.revealLocation.append(strong, ".");

    const sorted = [...d.results].sort((a, b) => b.score - a.score);
    els.revealResults.replaceChildren(...sorted.map((r) => {
        const li = document.createElement("li");
        li.className = "reveal__row";

        const name = document.createElement("span");
        name.className = "reveal__row-name";
        name.textContent = r.name;
        if (r.name === myName || d.results.length === 1) {
            totalScore = r.total;
            els.hudScore.textContent = totalScore.toLocaleString();
        }

        const points = document.createElement("span");
        points.className = "reveal__row-points";
        points.textContent = r.guessed ? `+${r.score.toLocaleString()}` : "no guess";
        if (r.guessed) {
            const dist = document.createElement("small");
            dist.textContent = formatDistance(r.distanceMeters);
            points.append(dist);
        }

        const bar = document.createElement("span");
        bar.className = "reveal__bar";
        const fill = document.createElement("span");
        fill.className = "reveal__bar-fill";
        bar.append(fill);
        // Set width on the next frame so the transition actually plays.
        requestAnimationFrame(() =>
            requestAnimationFrame(() => { fill.style.width = `${(r.score / MAX_SCORE) * 100}%`; }));

        li.append(name, points, bar);
        return li;
    }));
}

function renderStandings(standings) {
    const sorted = [...standings].sort((a, b) => b.total - a.total);
    els.finalHeadline.textContent =
        sorted.length > 1 ? `${sorted[0].name} takes it` : `${sorted[0].total.toLocaleString()} points`;

    els.standings.replaceChildren(...sorted.map((p, i) => {
        const li = document.createElement("li");
        const rank = document.createElement("span");
        rank.className = "standings__rank";
        rank.textContent = `${i + 1}.`;
        const name = document.createElement("span");
        name.className = "standings__name";
        name.textContent = p.name;
        const score = document.createElement("span");
        score.className = "standings__score";
        score.textContent = p.total.toLocaleString();
        li.append(rank, name, score);
        return li;
    }));
}

// ---------- user actions ----------

els.guessBtn.addEventListener("click", () => {
    if (!pendingGuess || hasGuessed) return;
    hasGuessed = true;
    send("GUESS", { lat: pendingGuess.lat, lng: pendingGuess.lng });
    els.guessBtn.disabled = true;
    els.guessBtn.textContent = "Guess locked — waiting for the round";
});

els.startBtn.addEventListener("click", () => {
    applyName();
    send("START");
});

els.nextBtn.addEventListener("click", () => send("NEXT"));

els.copyBtn.addEventListener("click", async () => {
    try {
        await navigator.clipboard.writeText(els.shareLink.value);
        toast("Invite link copied");
    } catch {
        els.shareLink.select();
        toast("Press ⌘C to copy");
    }
});

function applyName() {
    const name = els.nameInput.value.trim().slice(0, 24);
    if (name && name !== myName) {
        myName = name;
        localStorage.setItem("rg-name", name);
        send("HELLO", { name });
    }
}
els.nameInput.value = myName;
els.nameInput.addEventListener("change", applyName);

els.rejoinBtn.addEventListener("click", () => location.reload());

els.againBtn.addEventListener("click", async () => {
    // Fresh game, same settings.
    const res = await fetch("/games/start?rounds=5");
    const d = await res.json();
    location.href = `${location.pathname}?id=${d.id}${soloMode ? "&solo=1" : ""}`;
});

// ---------- boot ----------

if (!params.get("id")) {
    // Landed here without a game: make one (solo) and reload with its id.
    fetch("/games/start?rounds=5")
        .then((res) => res.json())
        .then((d) => location.replace(`${location.pathname}?id=${d.id}&solo=1`))
        .catch(() => {
            els.disconnectedReason.textContent = "Could not reach the game server.";
            setState("disconnected");
        });
} else {
    connect();
}
