package main

const (
	IMAGE_DIR = "./public/images"
)

// SQL Code
const (
	QUERY_USER_EXISTS = "SELECT EXISTS(SELECT 1 FROM users WHERE username = $1 OR email = $2)"
	QUERY_USERS_PSWRD = "SELECT password FROM users WHERE username = $1"
	QUERT_SESSION_TKN = "SELECT session_token, csrf_token FROM users WHERE session_token = $1"
	INSERT_USER_PSWRD = "INSERT INTO users (username, email, password) VALUES ($1, $2, $3)"
	SET_SESSION_TOKEN = "UPDATE users SET session_token = $1, csrf_token = $2 WHERE username = $3"
	CLR_SESSION_TOKEN = "UPDATE users SET session_token = '', csrf_token = '' WHERE session_token = $1"

	INSERT_LOCATION = "INSERT INTO locations (filename, name, latitude, longitude) VALUES ($1, $2, $3, $4)"
)
