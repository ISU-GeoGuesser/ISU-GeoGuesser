package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"

	gometadata "github.com/FlavioCFOliveira/GoMetadata"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

var db *sql.DB

type Authenticator interface {
	Middleware() gin.HandlerFunc
}

// --- GitHub OAuth authenticato ---

type GitHubAuthenticator struct {
	oauthConfig   *oauth2.Config
	sessionSecret string

	// replace with database
	sessions map[string]GitHubUser

	// organization string
}

type GitHubUser struct {
	Login string `json:"login"`
	Email string `json:"email"`
	ID    int    `json:"id"`
}

func NewGitHubAuthenticator(clientID, clientSecret, sessionSecret string) *GitHubAuthenticator {
	return &GitHubAuthenticator{
		oauthConfig: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes:       []string{"user:email", "read:org"},
			Endpoint:     github.Endpoint,
			RedirectURL:  "http://localhost:8080/auth/github/callback",
		},
		sessionSecret: sessionSecret,
		sessions:      make(map[string]GitHubUser),
		// organization:  organizationName,
	}
}

// func (a *GitHubAuthenticator) isOrgMember(client *http.Client, username string) (bool, error) {
// 	url := fmt.Sprintf("https://api.github.com/orgs/%s/members/%s", a.organization, username)
// 	resp, err := client.Get(url)
// 	if err != nil {
// 		return false, err
// 	}
// 	resp.Body.Close()

// 	return resp.StatusCode == http.StatusNoContent, nil
// }

func (a *GitHubAuthenticator) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing Authorization header"})
			return
		}

		user, ok := a.sessions[token]
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired session"})
			return
		}

		c.Set("github_user", user)
		c.Next()
	}
}

func (a *GitHubAuthenticator) RegisterRoutes(r *gin.Engine) {
	r.GET("/auth/github/login", func(c *gin.Context) {
		url := a.oauthConfig.AuthCodeURL("state", oauth2.AccessTypeOnline)
		c.Redirect(http.StatusTemporaryRedirect, url)
	})

	r.GET("/auth/github/callback", func(c *gin.Context) {
		code := c.Query("code")
		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing code"})
			return
		}

		token, err := a.oauthConfig.Exchange(context.Background(), code)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to exchange token"})
			return
		}

		client := a.oauthConfig.Client(context.Background(), token)
		resp, err := client.Get("https://api.github.com/user")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user"})
			return
		}
		defer resp.Body.Close()

		var user GitHubUser
		if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decode user"})
			return
		}

		// isMember, err := a.isOrgMember(a.oauthConfig.Client(context.Background(), token), user.Login)
		// if err != nil {
		// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check org membership"})
		// 	return
		// }
		// if !isMember {
		// 	c.JSON(http.StatusForbidden, gin.H{"error": fmt.Sprintf("you must be a member of %s", a.organization)})
		// 	return
		// }

		a.sessions[token.AccessToken] = user

		c.JSON(http.StatusOK, gin.H{
			"token":    token.AccessToken,
			"username": user.Login,
			"email":    user.Email,
		})
	})
}

func uploadLocation(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No image received: " + err.Error()})
		return
	}

	dst := "./images/" + file.Filename
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	m, err := gometadata.ReadFile(dst)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read metadata: " + err.Error()})
		return
	}

	response := gin.H{
		"message":  "Image uploaded successfully",
		"filename": file.Filename,
		"size":     file.Size,
	}

	lat, lon := 0.0, 0.0
	if gpsLat, gpsLon, ok := m.GPS(); ok {
		lat, lon = gpsLat, gpsLon
		response["latitude"] = lat
		response["longitude"] = lon
	}

	// _, err = db.Exec(
	// 	"INSERT INTO locations (filename, latitude, longitude) VALUES ($1, $2, $3)",
	// 	file.Filename, lat, lon,
	// )
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save location"})
	// 	return
	// }

	c.JSON(http.StatusOK, response)
}

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

	gamesAddRoutes(r)

	// Static frontend pages. Run the server from the repo root
	// (`go run ./backend`) so these relative paths resolve.
	r.Static("/home", "./home")
	r.Static("/game", "./game")
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/home/")
	})

	if err := r.Run(); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
