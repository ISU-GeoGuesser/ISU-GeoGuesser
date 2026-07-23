package main

import (
	db "isu-geoguesser/database"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func getLeaderboard(c *gin.Context) {
	limit, err := strconv.Atoi(c.GetHeader("number_of_players"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid number of players"})
		return
	}
	rows, err := db.GetTopPlayers(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get top players"})
		return
	}
	defer rows.Close()

	players := []map[string]interface{}{}
	for rows.Next() {
		var username string
		var score int
		if err := rows.Scan(&username, &score); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to scan top players"})
			return
		}
		players = append(players, gin.H{"username": username, "score": score})
	}
	c.JSON(http.StatusOK, players)
}
