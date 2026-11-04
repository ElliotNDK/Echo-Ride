package middleware

import (
	"echo-ride/pkg-library/errs"
	"echo-ride/services/auth-service/internal/infrastructure/redis"
	"echo-ride/services/auth-service/pkg/jwt"
	"strings"

	"github.com/labstack/echo/v5"
)

func JWTAuth(maker *jwt.TokenMaker, blacklist redis.Blacklist) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return errs.ErrUnauthorized.WithMessage("Missing Authorization header")
			}
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				return errs.ErrUnauthorized.WithMessage("Invalid Authorization header")
			}

			claims, err := maker.ParseAndVerify(parts[1])
			if err != nil {
				return errs.ErrUnauthorized.WithMessage("Invalid token").WithRootErr(err)
			}

			if claims.JTI != "" {
				exists, err := blacklist.Exists(c.Request().Context(), claims.JTI)
				if err == nil && exists {
					return errs.ErrUnauthorized.WithMessage("Token revoked")
				}
			}

			c.Set("userId", claims.UserID.String())
			c.Set("userRole", claims.Role)
			c.Set("jti", claims.JTI)
			if claims.ExpiresAt != nil {
				c.Set("accessExp", claims.ExpiresAt.Time)
			}

			return next(c)
		}
	}
}
