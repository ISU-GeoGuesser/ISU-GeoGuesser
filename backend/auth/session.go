package auth

import (
	"crypto/subtle"
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	db "isu-geoguesser/database"
)

var auth_err = errors.New("Unauthorized")

func authorize(c *gin.Context) error {
	// get session token from cookies
	st, err := c.Cookie("session_token")
	if err != nil || st == "" {
		return auth_err
	}

	// get csrf token from header
	csrf := c.GetHeader("X-CSRF-Token")
	if csrf == "" {
		return auth_err
	}

	// get tokens from db by session token
	var dbSession, dbCSRF string
	err = db.DB.QueryRow(db.QUERT_SESSION_TKN, st).Scan(&dbSession, &dbCSRF)
	if err == sql.ErrNoRows {
		return auth_err
	}
	if err != nil {
		return err
	}

	if subtle.ConstantTimeCompare([]byte(st), []byte(dbSession)) != 1 ||
		subtle.ConstantTimeCompare([]byte(csrf), []byte(dbCSRF)) != 1 {
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
