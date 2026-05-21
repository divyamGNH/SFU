package jwt

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserId string `json:"userId"`
	Email  string `json:"email,omitempty"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
}

var (
	ErrTokenExpired   = errors.New("token has expired")
	ErrTokenInvalid   = errors.New("token is invalid")
	ErrTokenMalformed = errors.New("token is malformed")
)

// Generate a new access token and return it.
func GenerateAccessToken(userId string, role string, email string, secret string, ttl time.Duration) (string, error) {
	now := time.Now()

	// Define the claims.
	claims := &Claims{
		UserId: userId,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userId,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    "D2CSFU",
		},
	}

	// This generates a new token body not the string yet
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Pass the secret as a array of bytes to finally get the string.
	return token.SignedString([]byte(secret))
}

// Validate a access token sent by the frontend.
func ValidateAccessToken(tokenString string, secret string) (*Claims, error) {
	// Decode the token
	// This automatically checks the expiry date and give err if needed.
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		// Ensure the token uses the
		_, ok := t.Method.(*jwt.SigningMethodHMAC)
		if !ok {
			return nil, fmt.Errorf("Unexpected signing method : %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})

	// Handle errors.
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		if errors.Is(err, jwt.ErrTokenMalformed) {
			return nil, ErrTokenMalformed
		}
		return nil, ErrTokenInvalid
	}

	// get the claims from the token.
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, ErrTokenInvalid
	}

	// return the claims
	return claims, nil
}

// Generate a new refresh token.
func GenerateRefreshToken() (string, error) {
	b := make([]byte, 48)

	// Fill the byte array with random bytes.
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Convert bytes to readable strings.
	return base64.URLEncoding.EncodeToString(b), nil
}
