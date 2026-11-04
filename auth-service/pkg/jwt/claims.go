package jwt

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type CustomClaims struct {
	Key    string    `json:"key"`
	UserID uuid.UUID `json:"sub"`
	Role   string    `json:"role"`
	Status string    `json:"status"`
	JTI    string    `json:"jti"`
	jwt.RegisteredClaims
}
