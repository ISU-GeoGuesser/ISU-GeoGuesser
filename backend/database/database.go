package database

import (
	"context"
	"database/sql"
	"isu-geoguesser/utils"
	"log"

	"gocloud.dev/postgres"
)

var DB *sql.DB

func Open() {
	ctx := context.Background()
	db, err := postgres.Open(ctx, utils.GetEnvFatal("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}

	DB = db
}

func Close() {
	DB.Close()
}

func GetTopPlayers(limit int) (*sql.Rows, error) {
	return DB.Query(QUERY_TOP_PLAYERS, limit)
}

type Location struct {
	Filename  string
	Latitude  float64
	Longitude float64
}

func GetRandomLocations(count int) ([]Location, error) {
	rows, err := DB.Query(RANDOM_LOCATIONS, count)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Location, 0, count)
	for rows.Next() {
		var loc Location
		if err := rows.Scan(&loc.Filename, &loc.Latitude, &loc.Longitude); err != nil {
			return nil, err
		}

		result = append(result, loc)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// SQL Code
const (
	QUERY_USER_EXISTS = "SELECT EXISTS(SELECT 1 FROM users WHERE username = $1 OR email = $2)"
	QUERY_USERS_PSWRD = "SELECT password FROM users WHERE username = $1"
	QUERT_SESSION_TKN = "SELECT session_token, csrf_token FROM users WHERE session_token = $1"
	QUERY_TOP_PLAYERS = "SELECT username, total_score FROM users ORDER BY total_score DESC LIMIT $1"
	INSERT_USER_PSWRD = "INSERT INTO users (username, email, password) VALUES ($1, $2, $3)"
	SET_SESSION_TOKEN = "UPDATE users SET session_token = $1, csrf_token = $2 WHERE username = $3"
	CLR_SESSION_TOKEN = "UPDATE users SET session_token = '', csrf_token = '' WHERE session_token = $1"

	INSERT_LOCATION  = "INSERT INTO locations (filename, name, latitude, longitude) VALUES ($1, $2, $3, $4)"
	RANDOM_LOCATIONS = "SELECT filename, latitude, longitude FROM locations ORDER BY random() LIMIT $1"
)
