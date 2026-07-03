package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Game pacing. Round length is configurable per game (?seconds= on
// /games/start) so demos and tests don't have to sit through 60s rounds.
const (
	defaultRounds       = 5
	minRounds           = 1
	maxRounds           = 10
	defaultRoundSeconds = 60
	minRoundSeconds     = 5
	maxRoundSeconds     = 180

	lobbyTimeout   = 5 * time.Minute  // lobby with nobody starting the game
	emptyTimeout   = 30 * time.Second // every player disconnected
	revealTimeout  = 90 * time.Second // nobody clicked "next round"
	gameOverLinger = 60 * time.Second // let players stare at final results
)

type gamePhase int

const (
	phaseLobby gamePhase = iota
	phaseGuessing
	phaseReveal
	phaseOver
)

type playerGuess struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type playerState struct {
	Conn   *webSocketConnection
	Name   string
	Total  int
	Guess  *playerGuess
	closed bool
}

func newPlayerState(conn *webSocketConnection, name string) *playerState {
	return &playerState{Conn: conn, Name: name}
}

func (p *playerState) Close() {
	if !p.closed {
		p.closed = true
		p.Conn.Close()
	}
}

func (p *playerState) Loop(game *gameMessenger) {
	for msg := range p.Conn.Rx {
		select {
		case game.PlayerMsg <- &playerMessage{p, msg}:
		case <-game.Done:
			return
		}
	}

	select {
	case game.PlayerLeft <- p:
	case <-game.Done:
	}
}

// SendMessage never blocks: if this player's connection is dead or too far
// behind to drain its buffered Tx channel, the message is dropped and the
// disconnect is handled when PlayerLeft arrives.
func (p *playerState) SendMessage(msg *webSocketMessage) {
	if p.closed {
		return
	}
	select {
	case p.Conn.Tx <- msg:
	default:
		log.Printf("Dropping message to stalled player %q", p.Name)
	}
}

type playerMessage struct {
	ply *playerState
	msg *webSocketMessage
}

type gameMessenger struct {
	NewConn    chan *webSocketConnection
	PlayerMsg  chan *playerMessage
	PlayerLeft chan *playerState
	Done       chan struct{}
}

func newGameMessenger() *gameMessenger {
	return &gameMessenger{
		NewConn:    make(chan *webSocketConnection),
		PlayerMsg:  make(chan *playerMessage),
		PlayerLeft: make(chan *playerState),
		Done:       make(chan struct{}),
	}
}

type gameState struct {
	ID uuid.UUID
	// Keyed by connection (NOT RemoteAddr — two players can share an
	// address behind a proxy or in tests, and would overwrite each other).
	Players map[*webSocketConnection]*playerState

	Phase        gamePhase
	Rounds       int
	RoundSeconds int
	Round        int // 1-based, 0 while in the lobby
	Locations    []gameLocation
	RoundEndsAt  time.Time
	playerSeq    int
}

func newGameState(id uuid.UUID, rounds, roundSeconds int) *gameState {
	return &gameState{
		ID:           id,
		Players:      make(map[*webSocketConnection]*playerState),
		Phase:        phaseLobby,
		Rounds:       rounds,
		RoundSeconds: roundSeconds,
	}
}

// --- wire payloads (server -> client) ---

type playerInfo struct {
	Name    string `json:"name"`
	Total   int    `json:"total"`
	Guessed bool   `json:"guessed"`
}

type roundStartData struct {
	Round        int    `json:"round"`
	Rounds       int    `json:"rounds"`
	Image        string `json:"image"`
	RoundSeconds int    `json:"roundSeconds"`
	EndsAtMs     int64  `json:"endsAtMs"`
}

type roundResultEntry struct {
	Name           string  `json:"name"`
	Guessed        bool    `json:"guessed"`
	Lat            float64 `json:"lat"`
	Lng            float64 `json:"lng"`
	DistanceMeters float64 `json:"distanceMeters"`
	Score          int     `json:"score"`
	Total          int     `json:"total"`
}

type roundResultData struct {
	Round   int                `json:"round"`
	Rounds  int                `json:"rounds"`
	Actual  gameLocation       `json:"actual"`
	Results []roundResultEntry `json:"results"`
}

func (s *gameState) playerList() []playerInfo {
	list := make([]playerInfo, 0, len(s.Players))
	for _, p := range s.Players {
		list = append(list, playerInfo{Name: p.Name, Total: p.Total, Guessed: p.Guess != nil})
	}
	return list
}

func (s *gameState) Broadcast(msg *webSocketMessage) {
	for _, p := range s.Players {
		p.SendMessage(msg)
	}
}

// resetTimer safely re-arms a timer whose channel is only ever drained by
// the game loop's select.
func resetTimer(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}

func (s *gameState) Loop(hub *gameHub, msg *gameMessenger) {
	log.Printf("[%s] Starting game loop (%d rounds, %ds each)", s.ID, s.Rounds, s.RoundSeconds)

	timer := time.NewTimer(lobbyTimeout)
	defer timer.Stop()

	done := false
	for !done {
		select {
		case m := <-msg.PlayerMsg:
			s.handleMessage(m.ply, m.msg, timer)
		case ply := <-msg.PlayerLeft:
			s.ForgetPlayer(ply)
			if len(s.Players) == 0 {
				resetTimer(timer, emptyTimeout)
			} else {
				s.Broadcast(newWebSocketMessageAssert("PLAYERS", s.playerList()))
				// The round shouldn't wait on someone who left.
				if s.Phase == phaseGuessing && s.allGuessed() {
					s.endRound(timer)
				}
			}
		case conn := <-msg.NewConn:
			s.AddPlayer(conn, msg)
		case <-timer.C:
			done = s.handleTimeout(timer)
		}
	}

	log.Printf("[%s] Game shutting down", s.ID)
	hub.ForgetGame(s.ID)
	close(msg.Done)

	for _, ply := range s.Players {
		s.ForgetPlayer(ply)
	}
}

// handleTimeout reacts to the phase timer firing; returns true when the
// game should shut down.
func (s *gameState) handleTimeout(timer *time.Timer) bool {
	if len(s.Players) == 0 {
		return true
	}

	switch s.Phase {
	case phaseLobby:
		return true // nobody started the game
	case phaseGuessing:
		s.endRound(timer) // time's up, lock in whatever guesses exist
	case phaseReveal:
		s.advance(timer) // nobody clicked next; move along
	case phaseOver:
		return true
	}
	return false
}

func (s *gameState) handleMessage(ply *playerState, msg *webSocketMessage, timer *time.Timer) {
	switch msg.Type {
	case "HELLO":
		var d struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(msg.Data, &d); err == nil && d.Name != "" {
			if len(d.Name) > 24 {
				d.Name = d.Name[:24]
			}
			ply.Name = d.Name
		}
		s.Broadcast(newWebSocketMessageAssert("PLAYERS", s.playerList()))

	case "START":
		if s.Phase != phaseLobby {
			return
		}
		s.Locations = pickRoundLocations(s.Rounds)
		s.startRound(timer)

	case "GUESS":
		if s.Phase != phaseGuessing || ply.Guess != nil {
			return
		}
		var d playerGuess
		if err := json.Unmarshal(msg.Data, &d); err != nil {
			ply.SendMessage(newWebSocketMessageAssert("ERROR", gin.H{"message": "bad guess payload"}))
			return
		}
		ply.Guess = &d
		s.Broadcast(newWebSocketMessageAssert("PLAYERS", s.playerList()))
		if s.allGuessed() {
			s.endRound(timer)
		}

	case "NEXT":
		if s.Phase != phaseReveal {
			return
		}
		s.advance(timer)

	default:
		log.Printf("[%s] Unknown message type %q from %s", s.ID, msg.Type, ply.Name)
	}
}

func (s *gameState) allGuessed() bool {
	for _, p := range s.Players {
		if p.Guess == nil {
			return false
		}
	}
	return len(s.Players) > 0
}

func (s *gameState) startRound(timer *time.Timer) {
	s.Round++
	s.Phase = phaseGuessing
	for _, p := range s.Players {
		p.Guess = nil
	}

	duration := time.Duration(s.RoundSeconds) * time.Second
	s.RoundEndsAt = time.Now().Add(duration)
	resetTimer(timer, duration)

	s.Broadcast(newWebSocketMessageAssert("ROUND_START", s.roundStartData()))
	log.Printf("[%s] Round %d/%d started: %s", s.ID, s.Round, s.Rounds, s.currentLocation().Name)
}

func (s *gameState) roundStartData() roundStartData {
	return roundStartData{
		Round:        s.Round,
		Rounds:       s.Rounds,
		Image:        s.currentLocation().Image,
		RoundSeconds: s.RoundSeconds,
		EndsAtMs:     s.RoundEndsAt.UnixMilli(),
	}
}

func (s *gameState) currentLocation() gameLocation {
	return s.Locations[s.Round-1]
}

func (s *gameState) endRound(timer *time.Timer) {
	actual := s.currentLocation()
	results := make([]roundResultEntry, 0, len(s.Players))

	for _, p := range s.Players {
		entry := roundResultEntry{Name: p.Name}
		if p.Guess != nil {
			entry.Guessed = true
			entry.Lat = p.Guess.Lat
			entry.Lng = p.Guess.Lng
			entry.DistanceMeters = haversineMeters(p.Guess.Lat, p.Guess.Lng, actual.Lat, actual.Lng)
			entry.Score = scoreForDistance(entry.DistanceMeters)
		}
		p.Total += entry.Score
		entry.Total = p.Total
		results = append(results, entry)
	}

	s.Phase = phaseReveal
	resetTimer(timer, revealTimeout)
	s.Broadcast(newWebSocketMessageAssert("ROUND_RESULT", roundResultData{
		Round:   s.Round,
		Rounds:  s.Rounds,
		Actual:  actual,
		Results: results,
	}))
	log.Printf("[%s] Round %d/%d resolved", s.ID, s.Round, s.Rounds)
}

func (s *gameState) advance(timer *time.Timer) {
	if s.Round >= s.Rounds {
		s.Phase = phaseOver
		resetTimer(timer, gameOverLinger)
		s.Broadcast(newWebSocketMessageAssert("GAME_OVER", gin.H{"standings": s.playerList()}))
		log.Printf("[%s] Game over", s.ID)
		return
	}
	s.startRound(timer)
}

func (s *gameState) AddPlayer(conn *webSocketConnection, msg *gameMessenger) {
	s.playerSeq++
	ply := newPlayerState(conn, fmt.Sprintf("Player %d", s.playerSeq))
	s.Players[conn] = ply
	go ply.Loop(msg)

	ply.SendMessage(newWebSocketMessageAssert("JOINED", gin.H{
		"you":          ply.Name,
		"roomCode":     s.ID.String(),
		"rounds":       s.Rounds,
		"roundSeconds": s.RoundSeconds,
		"inProgress":   s.Phase != phaseLobby,
		"players":      s.playerList(),
	}))

	// Late joiners drop straight into the round that's being played.
	if s.Phase == phaseGuessing {
		ply.SendMessage(newWebSocketMessageAssert("ROUND_START", s.roundStartData()))
	}

	s.Broadcast(newWebSocketMessageAssert("PLAYERS", s.playerList()))
	log.Printf("[%s] %s joined as %q", s.ID, conn.conn.RemoteAddr(), ply.Name)
}

func (s *gameState) ForgetPlayer(ply *playerState) {
	delete(s.Players, ply.Conn)
	ply.Close()
	log.Printf("[%s] %s (%q) left the game", s.ID, ply.Conn.conn.RemoteAddr(), ply.Name)
}

type gameHub struct {
	m sync.Map
}

func newGameHub() gameHub {
	return gameHub{}
}

func (h *gameHub) NewGame(rounds, roundSeconds int) uuid.UUID {
	id := uuid.New()
	msg := newGameMessenger()
	state := newGameState(id, rounds, roundSeconds)
	go state.Loop(h, msg)

	h.m.Store(id, msg)
	return id
}

func (h *gameHub) FindGame(id uuid.UUID) *gameMessenger {
	result, found := h.m.Load(id)
	if found {
		return result.(*gameMessenger)
	}
	return nil
}

func (h *gameHub) ForgetGame(id uuid.UUID) {
	h.m.Delete(id)
}

// clampQueryInt reads an integer query param and clamps it to [min, max].
func clampQueryInt(ctx *gin.Context, name string, def, min, max int) int {
	v, err := strconv.Atoi(ctx.Query(name))
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func gamesAddRoutes(eng *gin.Engine) {
	hub := newGameHub()

	games := eng.Group("/games")
	games.GET("/start", func(ctx *gin.Context) {
		rounds := clampQueryInt(ctx, "rounds", defaultRounds, minRounds, maxRounds)
		seconds := clampQueryInt(ctx, "seconds", defaultRoundSeconds, minRoundSeconds, maxRoundSeconds)
		id := hub.NewGame(rounds, seconds)
		ctx.JSON(http.StatusOK, gin.H{
			"id": id.String(),
		})
	})

	games.GET("/:id", func(ctx *gin.Context) {
		id, err := uuid.Parse(ctx.Param("id"))
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid ID",
			})
			return
		}

		game := hub.FindGame(id)
		if game == nil {
			ctx.JSON(http.StatusNotFound, gin.H{
				"error": "Game does not exist",
			})
			return
		}

		conn, err := openWebSocket(ctx)
		if err != nil {
			log.Printf("Failed to open WebSocket: %v", err)
			return
		}

		select {
		case game.NewConn <- conn:
		case <-game.Done:
			conn.Close()
		}
	})
}
