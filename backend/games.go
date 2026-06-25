package main

import (
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
)

type playerState struct {
	conn *webSocketConnection
}

func newPlayer(conn *webSocketConnection) *playerState {
	return &playerState{conn}
}

type gameState struct {
	mtx     sync.RWMutex
	players map[net.Addr]*playerState
}

func newGame() *gameState {
	return &gameState{}
}

func (s *gameState) gameAddPlayer(conn *webSocketConnection) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.players[conn.conn.RemoteAddr()] = newPlayer(conn)
}

func gamesAddRoutes(eng *gin.Engine) {
	gameCache := cache.New(10*time.Minute, 5*time.Minute)

	games := eng.Group("/games")
	games.GET("/start", func(ctx *gin.Context) {
		id := uuid.New().String()
		gameCache.SetDefault(id, newGame())

		ctx.JSON(http.StatusOK, gin.H{
			"id": id,
		})
	})

	games.GET("/:id", func(ctx *gin.Context) {
		id := ctx.Param("id")
		game, found := gameCache.Get(id)
		if !found {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"error": "Game does not exist",
			})
			return
		}

		conn, err := openWebSocket(ctx)
		if err != nil {
			log.Printf("Failed to open WebSocket: %v", err)
			return
		}

		game.(*gameState).gameAddPlayer(conn)
		log.Printf("%s joined game %s", conn.conn.RemoteAddr(), id)
	})
}
