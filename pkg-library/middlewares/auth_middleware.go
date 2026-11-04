package middlewares

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

func RequireAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			userID := c.Request().Header.Get("X-User-Id")
			if userID == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "X-User-Id header missing")
			}

			userRole := c.Request().Header.Get("X-User-Role")
			c.Set("userId", userID)
			c.Set("userRole", userRole)

			return next(c)
		}
	}
}

func RequireRole(allowedRoles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			userRole, ok := c.Get("userRole").(string)
			if !ok || userRole == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "User role not found")
			}

			for _, allowedRole := range allowedRoles {
				if userRole == allowedRole {
					return next(c)
				}
			}

			return echo.NewHTTPError(http.StatusForbidden, "Insufficient permissions")
		}
	}
}
