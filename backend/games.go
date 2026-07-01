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

func (p *playerState) Close() {
	p.Conn.Close()
}

func (p *playerState) Loop(game *gameMessenger) {
	for msg := range p.Conn.Rx {
		game.PlayerMsg <- &struct {
			ply *playerState
			msg *webSocketMessage
		}{p, msg}
	}

	game.PlayerLeft <- p
}

func (p *playerState) SendMessage(msg *webSocketMessage) {
	p.Conn.Tx <- msg
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
	NewConn   chan *webSocketConnection
	PlayerMsg chan *struct {
		ply *playerState
		msg *webSocketMessage
	}
	PlayerLeft chan *playerState
}

func newGameMessenger() *gameMessenger {
	return &gameMessenger{
		NewConn: make(chan *webSocketConnection),
		PlayerMsg: make(chan *struct {
			ply *playerState
			msg *webSocketMessage
		}),
		PlayerLeft: make(chan *playerState),
	}
}

func (s *gameState) Loop(hub *gameHub, msg *gameMessenger) {
	log.Printf("[%s] Starting game loop", s.ID)

out:
	for {
		select {
		case msg := <-msg.PlayerMsg:
			addr := msg.ply.Conn.conn.RemoteAddr()
			log.Printf("[%s] %s: %v", s.ID, addr, *msg.msg)
		case ply := <-msg.PlayerLeft:
			s.ForgetPlayer(ply)
		case conn := <-msg.NewConn:
			s.AddPlayer(conn, msg)
		case <-time.After(15 * time.Second):
			break out
		}
	}

	log.Printf("[%s] Game shutting down", s.ID)
	hub.ForgetGame(s.ID)

	for _, ply := range s.Players {
		s.ForgetPlayer(ply)
	}
}

func (s *gameState) AddPlayer(conn *webSocketConnection, msg *gameMessenger) {
	addr := conn.conn.RemoteAddr()
	ply := newPlayerState(conn)
	s.Players[addr] = ply
	go ply.Loop(msg)

	ply.SendMessage(newWebSocketMessageAssert("JOINED", nil))
	log.Printf("[%s] %s joined the game", s.ID, addr)
}

func (s *gameState) ForgetPlayer(ply *playerState) {
	addr := ply.Conn.conn.RemoteAddr()
	delete(s.Players, addr)
	ply.Close()
	log.Printf("[%s] %s left the game", s.ID, addr)
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
