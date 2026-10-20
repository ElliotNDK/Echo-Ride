package jwt

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type AccessTokenResult struct {
	Token     string
	JTI       string
	ExpiresAt time.Time
}

type RefreshTokenResult struct {
	Token     string
	Hash      string
	ExpiresAt time.Time
}

type TokenMaker struct {
	secretKey string
}

func NewTokenMaker(secretKey string) *TokenMaker {
	return &TokenMaker{secretKey: secretKey}
}

func (m *TokenMaker) GenerateAccessToken(userID uuid.UUID, role, status string, duration time.Duration) (*AccessTokenResult, error) {
	jti := uuid.NewString()
	expiresAt := time.Now().Add(duration)

	claims := CustomClaims{
		Key:    "echo_ride",
		UserID: userID,
		Role:   role,
		Status: status,
		JTI:    jti,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "auth-service",
			Subject:   userID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(m.secretKey))
	if err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	return &AccessTokenResult{
		Token:     signed,
		JTI:       jti,
		ExpiresAt: expiresAt,
	}, nil
}

func (m *TokenMaker) GenerateRefreshToken(duration time.Duration) (*RefreshTokenResult, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("failed to read random bytes: %w", err)
	}
	token := hex.EncodeToString(raw)
	hash := HashToken(token)
	return &RefreshTokenResult{
		Token:     token,
		Hash:      hash,
		ExpiresAt: time.Now().Add(duration),
	}, nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (m *TokenMaker) ParseAndVerify(tokenStr string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &CustomClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(m.secretKey), nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}
	claims, ok := token.Claims.(*CustomClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
