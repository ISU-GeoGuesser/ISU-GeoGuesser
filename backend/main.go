package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	games "isu-geoguesser/games"

	"isu-geoguesser/auth"
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

	auth.AddRoutes(r)

	locations := r.Group("/locations").Use(auth.AuthorizeMiddleware())
	{
		locations.POST("", uploadLocation)
	}

	games.AddRoutes(r)

	if err := r.Run(":3000"); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
