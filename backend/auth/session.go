package auth

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

var auth_err = errors.New("Unauthorized")

type tokenEntry struct {
	sessionToken string
	csrfToken    string
}

var (
	tokenStoreMu sync.RWMutex
	tokenStore   = make(map[string]tokenEntry) // keyed by session token
)

func storeTokens(sessionToken, csrfToken string) {
	tokenStoreMu.Lock()
	defer tokenStoreMu.Unlock()
	tokenStore[sessionToken] = tokenEntry{sessionToken: sessionToken, csrfToken: csrfToken}
}

func deleteToken(sessionToken string) {
	tokenStoreMu.Lock()
	defer tokenStoreMu.Unlock()
	delete(tokenStore, sessionToken)
}

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

	// look up tokens in memory
	tokenStoreMu.RLock()
	entry, ok := tokenStore[st]
	tokenStoreMu.RUnlock()
	if !ok {
		return auth_err
	}

	if subtle.ConstantTimeCompare([]byte(st), []byte(entry.sessionToken)) != 1 ||
		subtle.ConstantTimeCompare([]byte(csrf), []byte(entry.csrfToken)) != 1 {
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
