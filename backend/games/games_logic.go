package games

import (
	"log"
	"maps"
	"math"
	"path"
	"time"

	"isu-geoguesser/config"
	"isu-geoguesser/database"
	utils "isu-geoguesser/utils"
	ws "isu-geoguesser/websocket"
)

type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

//
// Client -> Server messages
//

// t: "GUESS"
//
// A guess is being made during the round.
type MessageGuess struct {
	Location
}

//
// Server -> Client messages
//

// t: "ROUND_STARTED"
//
// A round just started, and this message shares information needed by the
// client for display.
type MessageRoundStarted struct {
	// The image URL that players need to guess the location of.
	ImageUrl string `json:"image_url"`
	// How long this round lasts before it automatically closes.
	// This is optional, and blank/null means there is no timer.
	Duration *float64 `json:"duration,omitempty"`
	// The round number (starts at 1)
	Counter int `json:"counter"`
}

// t: "ROUND_OVER"
//
// The round just ended, and this message tells the player the results.
type MessageRoundOver struct {
	Score int `json:"score"`
	*Location
}

// t: "GAME_OVER"
//
// The game ended, and the lobby will close shortly.
type MessageGameOver struct {
	// How many total points you scored this game.
	TotalScore int `json:"total_score"`
}

//
// Game logic
//

type GamePhase int

const (
	// Waiting for players to join the lobby
	GamePhaseWaitingForPlayers GamePhase = iota
	// A round is active
	GamePhaseRound
	// Grace period after a round is over
	GamePhaseRoundOver
	// Game is over and the lobby will shut down soon
	GamePhaseOver
)

// Can return nil for a round with no timer
func (s *gameState) NextRoundDuration() *float64 {
	// TODO: this can change based on game settings
	return nil
}

func (s *gameState) NextRoundTimer() <-chan time.Time {
	seconds := s.NextRoundDuration()
	if seconds != nil {
		duration := time.Duration(*seconds * float64(time.Second))
		return time.NewTimer(duration).C
	} else {
		return nil
	}
}

// Called to advance the game state when the phase timer expires
//
// Returns true if the game is finished
func (s *gameState) NextGamePhase() bool {
	switch s.Phase {
	case GamePhaseWaitingForPlayers:
		s.StartRound()
	case GamePhaseRound:
		s.EndRound()
	case GamePhaseRoundOver:
		s.StartRound()
	case GamePhaseOver:
		return true
	}

	return false
}

func (s *gameState) GetCurrentDBLocation() *database.Location {
	return &s.Locations[s.RoundCounter-1]
}

func (s *gameState) GetCurrentLocation() Location {
	loc := s.GetCurrentDBLocation()
	return Location{
		Latitude:  loc.Latitude,
		Longitude: loc.Longitude,
	}
}

func (s *gameState) StartRound() {
	if s.RoundCounter >= s.RoundTotal {
		s.EndGame()
		return
	}

	s.Phase = GamePhaseRound
	s.PhaseTimer = s.NextRoundTimer()
	s.RoundCounter += 1
	log.Printf("[%s] Round %d starting", s.ID, s.RoundCounter)

	url := path.Join(config.IMAGE_URL_PREFIX, s.GetCurrentDBLocation().Filename)
	msg := ws.NewMessageAssert("ROUND_STARTED", &MessageRoundStarted{
		ImageUrl: url,
		Duration: nil,
		Counter:  s.RoundCounter,
	})

	for _, ply := range s.Players {
		// new rounds have to clear everyone's guesses
		ply.Guess = nil
		ply.SendMessage(msg)
	}
}

var campusBottomLeft = Location{40.502612, -89.000830}
var campusTopRight = Location{40.520390, -88.982966}
var campusHeight = campusTopRight.Latitude - campusBottomLeft.Latitude
var campusWidth = campusTopRight.Longitude - campusBottomLeft.Longitude
var campusMaxDistSqr = utils.Squaref(campusHeight*0.1) + utils.Squaref(campusWidth*0.1)

func calcGuessScore(ref *Location, guess *Location) int {
	diffLat := guess.Latitude - ref.Latitude
	diffLong := guess.Longitude - ref.Longitude
	distSqr := utils.Squaref(diffLat) + utils.Squaref(diffLong)

	frac := 1.0 - math.Min(1.0, distSqr/campusMaxDistSqr)
	return int(math.Ceil(frac * 5000))
}

func (s *gameState) EndRound() {
	s.Phase = GamePhaseRoundOver
	s.PhaseTimer = time.NewTimer(5 * time.Second).C
	log.Printf("[%s] Round %d ended", s.ID, s.RoundCounter)

	loc := s.GetCurrentLocation()
	for _, ply := range s.Players {
		score := 0
		if ply.Guess != nil {
			score = calcGuessScore(&loc, ply.Guess)
		}

		msg := ws.NewMessageAssert("ROUND_OVER", &MessageRoundOver{
			Score:    score,
			Location: &loc,
		})
		ply.SendMessage(msg)
		ply.TotalScore += score
	}
}

func (s *gameState) EndGame() {
	s.Phase = GamePhaseOver
	s.PhaseTimer = time.NewTimer(10 * time.Second).C
	log.Printf("[%s] Game ended", s.ID)

	for _, ply := range s.Players {
		msg := ws.NewMessageAssert("GAME_OVER", &MessageGameOver{
			TotalScore: ply.TotalScore,
		})
		ply.SendMessage(msg)
	}
}

func (s *gameState) HandlePlayerGuess(ply *playerState, m *MessageGuess) {
	success := false
	// can't make a guess outside of a round
	if s.Phase == GamePhaseRound {
		ply.Guess = &m.Location
		success = true
	}

	// TODO: this can probably be replaced by a "GUESSED" message broadcast to
	// everyone when a player makes a guess
	ply.SendMessage(ws.NewMessageAssert("GUESS", success))

	// if every player guessed, we advance the round early
	if s.Phase == GamePhaseRound {
		madeGuess := func(p *playerState) bool { return p.Guess != nil }
		if utils.All(maps.Values(s.Players), madeGuess) {
			s.EndRound()
		}
	}
}
