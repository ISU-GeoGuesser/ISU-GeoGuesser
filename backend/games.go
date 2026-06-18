package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
)

func gamesAddRoutes(eng *gin.Engine) {
	gameCache := cache.New(10*time.Minute, 5*time.Minute)

	games := eng.Group("/games")
	games.GET("/start", func(ctx *gin.Context) {
		id := uuid.New().String()
		gameCache.SetDefault(id, nil)

		ctx.JSON(http.StatusOK, gin.H{
			"id": id,
		})
	})

	games.GET("/:id", func(ctx *gin.Context) {
		_, found := gameCache.Get(ctx.Param("id"))
		if !found {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"error": "Game does not exist",
			})
		} else {
			handleWebSocket(ctx)
		}
	})
}
