package auth

import (
	"crypto/subtle"
	"database/sql"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	db "isu-geoguesser/database"
)

var auth_err = errors.New("Unauthorized")

func authorize(c *gin.Context) error {
	// get session token from cookies
	st, err := c.Cookie("session_token")
	if err != nil || st == "" {
		log.Println("Missing or empty session token in cookies")
		return auth_err
	}

	// get csrf token from header
	csrf := c.GetHeader("X-CSRF-Token")
	if csrf == "" {
		log.Println("Missing CSRF token in header")
		return auth_err
	}

	// get tokens from db by session token
	var dbSession, dbCSRF string
	err = db.DB.QueryRow(db.QUERT_SESSION_TKN, st).Scan(&dbSession, &dbCSRF)
	if err == sql.ErrNoRows {
		log.Println("No session found for session token")
		return auth_err
	}
	if err != nil {
		log.Println("Error querying session token:", err)
		return err
	}

	if subtle.ConstantTimeCompare([]byte(st), []byte(dbSession)) != 1 ||
		subtle.ConstantTimeCompare([]byte(csrf), []byte(dbCSRF)) != 1 {
		log.Println("Session token or CSRF token mismatch")
		return auth_err
	}

	return nil
}

func AuthorizeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := authorize(c); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		c.Next()
	}
}
