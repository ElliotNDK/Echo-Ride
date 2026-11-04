package http

import (
	"echo-ride/pkg-library/errs"
	"echo-ride/pkg-library/response"
	"echo-ride/services/auth-service/internal/application"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
)

type AuthHandler struct {
	registerUC application.RegisterUseCase
	loginUC    application.LoginUseCase
	refreshUC  application.RefreshUseCase
	logoutUC   application.LogoutUseCase
}

func NewAuthHandler(
	e *echo.Echo,
	registerUC application.RegisterUseCase,
	loginUC application.LoginUseCase,
	refreshUC application.RefreshUseCase,
	logoutUC application.LogoutUseCase,
	jwtAuth echo.MiddlewareFunc,
) {
	handler := &AuthHandler{
		registerUC: registerUC,
		loginUC:    loginUC,
		refreshUC:  refreshUC,
		logoutUC:   logoutUC,
	}

	v1 := e.Group("/api/v1/auth")
	v1.POST("/register", handler.Register)
	v1.POST("/login", handler.Login)
	v1.POST("/refresh", handler.Refresh)
	v1.POST("/logout", handler.Logout, jwtAuth)
}

func (h *AuthHandler) Register(ctx *echo.Context) error {
	var req registerRequest
	if err := ctx.Bind(&req); err != nil {
		return errs.ErrBadRequest.WithMessage("Invalid request body").WithRootErr(err)
	}
	if err := ctx.Validate(&req); err != nil {
		return errs.ErrBadRequest.WithMessage("Validation failed: " + err.Error())
	}

	resp, err := h.registerUC.Execute(ctx.Request().Context(), application.RegisterRequest{
		Email:    req.Email,
		Phone:    req.Phone,
		Password: req.Password,
		Role:     req.Role,
	})
	if err != nil {
		return err
	}

	return response.WriteSuccess(ctx, http.StatusCreated, resp, "Account created successfully")
}

func (h *AuthHandler) Login(ctx *echo.Context) error {
	var req loginRequest
	if err := ctx.Bind(&req); err != nil {
		return errs.ErrBadRequest.WithMessage("Invalid request body")
	}
	if err := ctx.Validate(&req); err != nil {
		return errs.ErrBadRequest.WithMessage("Validation failed: " + err.Error())
	}

	res, err := h.loginUC.Execute(ctx.Request().Context(), application.LoginRequest{
		Phone:      req.Phone,
		Password:   req.Password,
		DeviceInfo: req.DeviceInfo,
		IPAddress:  ctx.RealIP(),
		UserAgent:  ctx.Request().UserAgent(),
	})
	if err != nil {
		return err
	}

	return response.WriteSuccess(ctx, http.StatusOK, res, "Login successful")
}

func (h *AuthHandler) Refresh(ctx *echo.Context) error {
	var req refreshRequest
	if err := ctx.Bind(&req); err != nil {
		return errs.ErrBadRequest.WithMessage("Invalid request body").WithRootErr(err)
	}
	if err := ctx.Validate(&req); err != nil {
		return errs.ErrBadRequest.WithMessage("Validation failed: " + err.Error())
	}

	res, err := h.refreshUC.Execute(ctx.Request().Context(), application.RefreshRequest{
		RefreshToken: req.RefreshToken,
		IPAddress:    ctx.RealIP(),
		UserAgent:    ctx.Request().UserAgent(),
	})
	if err != nil {
		return err
	}

	return response.WriteSuccess(ctx, http.StatusOK, res, "Token refreshed")
}

func (h *AuthHandler) Logout(ctx *echo.Context) error {
	var req logoutRequest
	if err := ctx.Bind(&req); err != nil {
		return errs.ErrBadRequest.WithMessage("Invalid request body").WithRootErr(err)
	}
	if err := ctx.Validate(&req); err != nil {
		return errs.ErrBadRequest.WithMessage("Validation failed: " + err.Error())
	}

	jti, _ := ctx.Get("jti").(string)
	exp, _ := ctx.Get("accessExp").(time.Time)

	if err := h.logoutUC.Execute(ctx.Request().Context(), application.LogoutRequest{
		RefreshToken: req.RefreshToken,
		JTI:          jti,
		AccessExp:    exp,
	}); err != nil {
		return err
	}

	return response.WriteSuccess(ctx, http.StatusOK, map[string]string{"status": "logged_out"}, "Logged out")
}
