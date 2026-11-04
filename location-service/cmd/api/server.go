package main

import (
	"echo-ride/pkg-library/response"
	"echo-ride/services/location-service/internal/presentation/ws"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"go.uber.org/zap"
)

type customValidator struct {
	v *validator.Validate
}

func (cv *customValidator) Validate(i interface{}) error {
	return cv.v.Struct(i)
}

func newServer(wsHandler *ws.Handler, log *zap.Logger) *echo.Echo {
	e := echo.New()
	e.Validator = &customValidator{v: validator.New()}
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))
	e.Use(middleware.Recover(), middleware.RequestID())

	e.HTTPErrorHandler = response.CustomHTTPErrorHandler(log)

	e.GET("/health", func(c *echo.Context) error {
		return response.WriteSuccess(c, http.StatusOK, map[string]string{"service": "location-service"}, "Health check successful")
	})

	e.GET("/ws", wsHandler.ServeWS)

	return e
}
