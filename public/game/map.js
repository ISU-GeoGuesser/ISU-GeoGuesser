
// Connect to the backend game via WebSocket.
// If no ?id= is present in the URL, create a new game first.
const backendUrl = new URL("https://api.reggieguessr.com");

let socket;

async function initGame() {
    let gameId = new URLSearchParams(window.location.search).get("id");

    if (!gameId) {
        const res = await fetch(new URL("/games/start", backendUrl));
        if (!res.ok) {
            alert("Failed to start a game. Is the backend running?");
            return;
        }
        const data = await res.json();
        gameId = data.id;
        // Update the URL so a refresh reconnects to the same game
        history.replaceState(null, "", `?id=${gameId}`);
    }

    let wsUrl = new URL(`/games/${gameId}`, backendUrl);
    wsUrl.protocol = backendUrl.protocol === "https" ? "wss" : "ws";
    socket = new WebSocket(wsUrl);
    socket.addEventListener("message", onMessage);
    socket.addEventListener("close", onClose);
}

initGame();

let playerGuess = null;
let guessMarker = null;
let actualMarker = null;

let campusLat = 40.508579;
let campusLng = -88.991159;

let map = L.map("map").setView([campusLat, campusLng], 16);

L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {maxZoom: 19,
attribution: "&copy; OpenStreetMap contributors"}).addTo(map);

map.on("click", function(event) {

    playerGuess = event.latlng;

    if (guessMarker !== null) {
        map.removeLayer(guessMarker);
    }

    guessMarker = L.marker([playerGuess.lat, playerGuess.lng]).addTo(map);
    guessMarker.bindPopup("Your Guess").openPopup();

});

let timeLeft = 60;
let timer;
let roundText = document.getElementById("roundText");
let timerText = document.getElementById("timer");
let scoreText = document.getElementById("scoreText");
let submitButton = document.getElementById("submitGuess");
let roundPhoto = document.querySelector(".roundPhoto");

submitButton.addEventListener("click", function() {

    if (playerGuess === null) {
        alert("Make a guess before submitting!");
        return;
    }

    if (!socket || socket.readyState !== WebSocket.OPEN) {
        alert("Not connected to game yet. Please wait.");
        return;
    }

    socket.send(JSON.stringify({
        t: "GUESS",
        d: {
            latitude: playerGuess.lat,
            longitude: playerGuess.lng
        }
    }));

});

// Start a visual countdown. If duration is null the backend has no timer,
// so we display infinity.
function startTimer(duration) {

    clearInterval(timer);

    if (!duration) {
        timerText.textContent = "\u221E";
        return;
    } else {
        timeLeft = duration;
        timerText.textContent = Math.ceil(duration);
    }

    timer = setInterval(function() {
        timeLeft -= 1;
        timerText.textContent = Math.ceil(timeLeft);

        if (timeLeft <= 0) {
            clearInterval(timer);
        }
    }, 1000);

}

function onMessage(event) {

    const msg = JSON.parse(event.data);

    switch (msg.t) {

        case "ROUND_STARTED": {
            const data = msg.d;

            // Update the campus photo
            if (roundPhoto) {
                let url = new URL(data.image_url, window.location.origin);
                roundPhoto.src = url;
            }

            // Round counter comes from the backend
            roundText.textContent = data.counter;

            startTimer(data.duration);

            // Clear previous markers
            playerGuess = null;

            if (guessMarker !== null) {
                map.removeLayer(guessMarker);
                guessMarker = null;
            }

            if (actualMarker !== null) {
                map.removeLayer(actualMarker);
                actualMarker = null;
            }

            submitButton.disabled = false;
            submitButton.textContent = "Submit Guess";
            break;
        }

        case "ROUND_OVER": {
            const data = msg.d;
            clearInterval(timer);

            // Update running score display
            if (scoreText) {
                scoreText.textContent = parseInt(scoreText.textContent, 10) + data.score;
            }

            // Show the actual location on the map
            actualMarker = L.marker([data.latitude, data.longitude])
                .addTo(map)
                .bindPopup(`Actual Location<br>Round score: ${data.score}`)
                .openPopup();

            submitButton.disabled = true;
            submitButton.textContent = "Waiting for next round\u2026";
            break;
        }

        case "GUESS": {
            // Server confirms whether the guess was recorded (bool)
            if (!msg.d) {
                alert("Guess was not accepted — the round may have already ended.");
            }
            break;
        }

    }

}

function onClose() {

    clearInterval(timer);
    roundText.textContent = "Game Over";
    timerText.textContent = "0";
    submitButton.disabled = true;
    submitButton.textContent = "Game Over";

}
