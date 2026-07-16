package main

import (
	"context"
	"database/sql"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"gocloud.dev/postgres"
)

var db *sql.DB

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
	ctx := context.Background()
	db, err := postgres.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// ---------------
	// -- gin stuff --
	r := gin.Default()

	r.POST("/register", register)
	r.POST("/login", login)
	r.POST("/logout", logout)

	locations := r.Group("/locations").Use(authorizeMiddleware())
	{
		locations.POST("", uploadLocation)
	}

	gamesAddRoutes(r)

	if err := r.Run(":3000"); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
