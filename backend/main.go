package main

import (
	"database/sql"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	games "isu-geoguesser/games"
)

var db *sql.DB

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, falling back to environment")
	}

	var auth Authenticator = NewGitHubAuthenticator(
		os.Getenv("GITHUB_CLIENT_ID"),
		os.Getenv("GITHUB_CLIENT_SECRET"),
		os.Getenv("SESSION_SECRET"),
		// os.Getenv("GITHUB_ORG_NAME"),
	)

	r := gin.Default()

	auth.(*GitHubAuthenticator).RegisterRoutes(r)

	locations := r.Group("/locations", auth.Middleware())
	{
		locations.POST("", uploadLocation)
	}

	games.AddRoutes(r)

	if err := r.Run(":3000"); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
