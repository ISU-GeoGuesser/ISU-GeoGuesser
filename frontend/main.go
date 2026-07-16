package main

import (
	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()

	// serve static pages
	router.Static("/home", "./public/home")
	router.Static("/leaderboard", "./public/leaderboard")
	router.Static("/images", "./public/images")
	// ...

	// redirect root to home
	router.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/home")
	})

	router.Run(":8080")
}
