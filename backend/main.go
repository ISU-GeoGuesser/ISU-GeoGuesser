package main

import (
	"isu-geoguesser/auth"
	"isu-geoguesser/utils"
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	games "isu-geoguesser/games"

	db "isu-geoguesser/database"
)

func main() {
	// load environment file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, falling back to environment")
	}

	// initialize authenticator
	// var auth Authenticator = NewGitHubAuthenticator(
	// 	os.Getenv("GITHUB_CLIENT_ID"),
	// 	os.Getenv("GITHUB_CLIENT_SECRET"),
	// 	os.Getenv("SESSION_SECRET"),
	// 	// os.Getenv("GITHUB_ORG_NAME"),
	// )

	// -----------------------------
	// -- open database (postgre) --
	db.Open()
	defer db.Close()

	// ---------------
	// -- gin stuff --
	r := gin.Default()

	frontendDomain := utils.GetEnvFatal("FRONTEND_DOMAIN")
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{frontendDomain},
		AllowMethods:     []string{"GET", "POST"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-CSRF-Token"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	auth.AddRoutes(r, frontendDomain)

	locations := r.Group("/locations").Use(auth.AuthorizeMiddleware())
	{
		locations.POST("", uploadLocation)
	}
	// r.POST("/locations", uploadLocation)

	r.GET("leaderboard", getLeaderboard)

	games.AddRoutes(r, frontendDomain)

	if err := r.Run(":3000"); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
