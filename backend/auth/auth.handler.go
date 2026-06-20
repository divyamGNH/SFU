package auth

import (
	"backend/models"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type AuthHandler struct {
	repo           AuthRepository
	jwtSecret      string
	accessTokenTTL time.Duration
	refreshTTL     time.Duration
	googleClientID string
}

type AuthConfig struct {
	JWTSecret      string
	AccessTokenTTL time.Duration
	RefreshTTL     time.Duration
	GoogleClientID string
}

// Pass the auth repo interface during initialization.
func NewAuthHandler(repo AuthRepository, cfg AuthConfig) *AuthHandler {
	if cfg.AccessTokenTTL == 0 {
		cfg.AccessTokenTTL = 15 * time.Minute
	}
	if cfg.RefreshTTL == 0 {
		cfg.RefreshTTL = 30 * 24 * time.Hour
	}
	return &AuthHandler{
		repo:           repo,
		jwtSecret:      cfg.JWTSecret,
		accessTokenTTL: cfg.AccessTokenTTL,
		refreshTTL:     cfg.RefreshTTL,
		googleClientID: cfg.GoogleClientID,
	}
}

// Send HTTP responses back.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: encode error: %v", err)
	}
}

// Send HTTP errors.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// Generate
func IssueTokenPair(w http.Response, r *http.Request, user *models.User) {

}
