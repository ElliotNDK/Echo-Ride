package main

import (
	"echo-ride/pkg-library/middlewares"
	"echo-ride/pkg-library/response"
	"echo-ride/services/auth-service/config"
	"echo-ride/services/auth-service/internal/application"
	authHttp "echo-ride/services/auth-service/internal/presentation/http"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
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

type ServerConfig struct {
	DBPool     *pgxpool.Pool
	Config     *config.Config
	Logger     *zap.Logger
	RegisterUC application.RegisterUseCase
	LoginUC    application.LoginUseCase
	RefreshUC  application.RefreshUseCase
	LogoutUC   application.LogoutUseCase
	JWTAuth    echo.MiddlewareFunc
}

func newServer(cfg ServerConfig) *echo.Echo {
	e := echo.New()
	e.Validator = &customValidator{v: validator.New()}
	e.Use(middleware.Recover())
	e.Use(middlewares.OTelMiddleware("auth-service"))
	e.HTTPErrorHandler = response.CustomHTTPErrorHandler(cfg.Logger)

	e.GET("/health", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok", "service": "auth-service"})
	})

	authHttp.NewAuthHandler(e, cfg.RegisterUC, cfg.LoginUC, cfg.RefreshUC, cfg.LogoutUC, cfg.JWTAuth)

	return e
}
