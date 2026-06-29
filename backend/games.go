package main

import (
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type playerState struct {
	Conn *webSocketConnection
}

func newPlayerState(conn *webSocketConnection) *playerState {
	return &playerState{Conn: conn}
}

type gameState struct {
	ID      uuid.UUID
	Players map[net.Addr]*playerState
}

func newGameState(id uuid.UUID) *gameState {
	return &gameState{
		ID:      id,
		Players: make(map[net.Addr]*playerState),
	}
}

type gameMessenger struct {
	NewConn chan *webSocketConnection
}

func newGameMessenger() *gameMessenger {
	return &gameMessenger{
		NewConn: make(chan *webSocketConnection),
	}
}

func (s *gameState) Loop(hub *gameHub, msg *gameMessenger) {
	log.Printf("[%s] Starting game loop", s.ID)

out:
	for {
		select {
		case conn := <-msg.NewConn:
			s.AddPlayer(conn)
		case <-time.After(15 * time.Second):
			break out
		}
	}

	hub.ForgetGame(s.ID)
	log.Printf("[%s] Game shutting down", s.ID)
}

func (s *gameState) AddPlayer(conn *webSocketConnection) {
	addr := conn.conn.RemoteAddr()
	s.Players[addr] = newPlayerState(conn)
	log.Printf("[%s] %s joined the game", s.ID, addr)
}

type gameHub struct {
	m sync.Map
}

func newGameHub() gameHub {
	return gameHub{}
}

func (h *gameHub) NewGame() uuid.UUID {
	id := uuid.New()
	msg := newGameMessenger()
	state := newGameState(id)
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

func gamesAddRoutes(eng *gin.Engine) {
	hub := newGameHub()

	games := eng.Group("/games")
	games.GET("/start", func(ctx *gin.Context) {
		id := hub.NewGame()
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

		game.NewConn <- conn
	})
}
