package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"

	db "isu-geoguesser/database"
)

type RegisterRequest struct {
	Username string `form:"username" binding:"required,min=8,max=32,alphanum"`
	Password string `form:"password" binding:"required,min=8"`
	Email    string `form:"email" binding:"required,email"`
}

func register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid username or password"})
		return
	}

	// hash password
	hashedPassword, err := hash_password(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// store user, email, and hashed password
	_, err = db.DB.Exec(db.INSERT_USER_PSWRD, req.Username, req.Email, hashedPassword)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			c.JSON(http.StatusConflict, gin.H{"error": "Username or email already taken"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Account created"})
}

func login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	if username == "" || password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid username or password"})
		return
	}

	// get hashed password from db
	var hashedPassword string
	err := db.DB.QueryRow(db.QUERY_USERS_PSWRD, username).Scan(&hashedPassword)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	if !checkPasswordHash(password, hashedPassword) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	sessionToken := generateToken(32)
	csrfToken := generateToken(32)

	// store tokens in db
	_, err = db.DB.Exec(db.SET_SESSION_TOKEN, sessionToken, csrfToken, username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	//                                       maxAge, path, domain, secure, httpOnly
	c.SetCookie("session_token", sessionToken, 86400, "/", "", true, true)
	c.SetCookie("csrf_token", csrfToken, 86400, "/", "", true, false)

	c.JSON(http.StatusOK, gin.H{"message": "Logged in"})
}

func logout(c *gin.Context) {
	if err := authorize(c); err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	st, _ := c.Cookie("session_token")

	// clear tokens from db
	_, err := db.DB.Exec(db.CLR_SESSION_TOKEN, st)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// clear session cookies
	c.SetCookie("session_token", "", -1, "/", "", true, true)
	c.SetCookie("csrf_token", "", -1, "/", "", true, false)

	c.JSON(http.StatusOK, gin.H{"message": "Logged out"})
}

func AddRoutes(r *gin.Engine) {
	r.POST("/register", register)
	r.POST("/login", login)
	r.POST("/logout", logout)
}
