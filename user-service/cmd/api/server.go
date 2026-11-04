package main

import (
	"echo-ride/pkg-library/middlewares"
	"echo-ride/pkg-library/response"
	"echo-ride/services/user-service/config"
	"echo-ride/services/user-service/internal/application"
	userHttp "echo-ride/services/user-service/internal/presentation/http"
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
	DBPool          *pgxpool.Pool
	Config          *config.Config
	Logger          *zap.Logger
	GetUserUC       application.GetUserUseCase
	UpdateProfileUC application.UpdateProfileUseCase
	UpdateRoleUC    application.UpdateRoleUseCase
	UpdateStatusUC  application.UpdateStatusUseCase
}

func newServer(cfg ServerConfig) *echo.Echo {
	e := echo.New()
	e.Validator = &customValidator{v: validator.New()}
	e.Use(middleware.Recover())
	e.Use(middlewares.OTelMiddleware("user-service"))
	e.HTTPErrorHandler = response.CustomHTTPErrorHandler(cfg.Logger)

	e.GET("/health", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok", "service": "user-service"})
	})

	userHttp.NewUserHandler(e, cfg.GetUserUC, cfg.UpdateProfileUC, cfg.UpdateRoleUC, cfg.UpdateStatusUC)

	return e
}
