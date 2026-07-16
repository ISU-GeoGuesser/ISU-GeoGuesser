package auth

import (
	"crypto/rand"
	"encoding/base64"
	"log"

	"golang.org/x/crypto/bcrypt"
)

func hash_password(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func checkPasswordHash(password, hash string) bool {
	return nil == bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func generateToken(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		log.Fatalf("Failed to generate token: %v", err)
	}
	return base64.URLEncoding.EncodeToString(bytes)
}
