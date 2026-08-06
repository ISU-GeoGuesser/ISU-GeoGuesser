
let playerGuess = null;
let guessMarker = null;

let campusLat = 40.508579;
let campusLng = -88.991159;

let map = L.map("map").setView([campusLat, campusLng], 16);

L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {maxZoom: 19, 
attribution: "&copy; OpenStreetMap contributors"}).addTo(map); 

map.on("click", function(event) {

    playerGuess = event.latlng; 

    if(guessMarker !== null){
        map.removeLayer(guessMarker);
    }

    guessMarker = L.marker([playerGuess.lat, playerGuess.lng]).addTo(map); 

    guessMarker.bindPopup("Your Guess").openPopup(); 

    console.log("Player Guessed"); 
    console.log("Latitude:", playerGuess.lat); 
    console.log("Longitude:", playerGuess.lng)

}); 

let currentRound = 1; 
let maxRounds = 5; 
let timeLeft = 60; 
let timer; 
let roundText = document.getElementById("roundText"); 
let timerText = document.getElementById("timer"); 
let submitButton = document.getElementById("submitGuess"); 


submitButton.addEventListener("click", function() {

    if(playerGuess == null) {
        alert("Make a guess before submitting!");
        return;
    }
    //send guesses to the backend (just making it pop up on screen for now idk how to merg yet lol)
    alert("Guess Submitted!\n" + "Latitude: " +playerGuess.lat.toFixed(5)+ "\n" + "Longitude: " + 
    playerGuess.lng.toFixed(5)); 

    nextRound(); 

}); 


function startTimer() { 

    clearInterval(timer); 
    timeLeft = 60; 
    timerText.textContent = timeLeft; 

    timer = setInterval(function() {
        timeLeft = timeLeft - 1; 
        timerText.textContent = timeLeft; 

        if(timeLeft <= 0) {
            clearInterval(timer); 
            nextRound(); 
        }

    }, 1000)

}

function nextRound() {

    currentRound = currentRound + 1; 
    if(currentRound > maxRounds) {

        endGame();
        return;

    }

    roundText.textContent = currentRound + "/" + maxRounds; 

    playerGuess = null; 

    if(guessMarker != null) {

        map.removeLayer(guessMarker); 
        guessMarker = null; 

    }

    startTimer()

}

function endGame() {

    clearInterval(timer); 
    roundText.textContent = "Game Over"; 
    timerText.textContent = "0"; 
    submitButton.disabled = true; 
    submitButton.textContent = "Game Over"; 

}

startTimer(); 