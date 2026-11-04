package http

import (
	"echo-ride/pkg-library/errs"
	"echo-ride/pkg-library/middlewares"
	"echo-ride/pkg-library/response"
	"echo-ride/services/user-service/internal/application"
	"echo-ride/services/user-service/internal/domain"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type UserHandler struct {
	getUserUC       application.GetUserUseCase
	updateProfileUC application.UpdateProfileUseCase
	updateRoleUC    application.UpdateRoleUseCase
	updateStatusUC  application.UpdateStatusUseCase
}

func NewUserHandler(
	e *echo.Echo,
	getUserUC application.GetUserUseCase,
	updateProfileUC application.UpdateProfileUseCase,
	updateRoleUC application.UpdateRoleUseCase,
	updateStatusUC application.UpdateStatusUseCase,
) {
	h := &UserHandler{
		getUserUC:       getUserUC,
		updateProfileUC: updateProfileUC,
		updateRoleUC:    updateRoleUC,
		updateStatusUC:  updateStatusUC,
	}

	v1 := e.Group("/api/v1/users")
	v1.Use(middlewares.RequireAuth())

	v1.GET("/me", h.GetMe)
	v1.PATCH("/me/profile", h.UpdateMyProfile)
	v1.GET("/:id", h.GetByID)

	admin := e.Group("/api/v1/admin/users")
	admin.Use(middlewares.RequireAuth(), middlewares.RequireRole("ADMIN"))
	admin.PATCH("/:id/role", h.UpdateRole)
	admin.PATCH("/:id/status", h.UpdateStatus)
}

func userIDFromContext(ctx *echo.Context) (uuid.UUID, error) {
	userIDStr, ok := ctx.Get("userId").(string)
	if !ok || userIDStr == "" {
		return uuid.Nil, errs.ErrUnauthorized.WithMessage("Missing user context")
	}
	id, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, errs.ErrUnauthorized.WithMessage("Invalid user id").WithRootErr(err)
	}
	return id, nil
}

func (h *UserHandler) GetMe(ctx *echo.Context) error {
	userID, err := userIDFromContext(ctx)
	if err != nil {
		return err
	}
	user, err := h.getUserUC.Execute(ctx.Request().Context(), userID)
	if err != nil {
		return err
	}
	return response.WriteSuccess(ctx, http.StatusOK, user, "User retrieved successfully")
}

func (h *UserHandler) GetByID(ctx *echo.Context) error {
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		return errs.ErrBadRequest.WithMessage("Invalid user ID").WithRootErr(err)
	}
	user, err := h.getUserUC.Execute(ctx.Request().Context(), id)
	if err != nil {
		return err
	}
	return response.WriteSuccess(ctx, http.StatusOK, user, "User retrieved successfully")
}

func (h *UserHandler) UpdateMyProfile(ctx *echo.Context) error {
	userID, err := userIDFromContext(ctx)
	if err != nil {
		return err
	}

	var req updateProfileRequest
	if err := ctx.Bind(&req); err != nil {
		return errs.ErrBadRequest.WithMessage("Invalid request body").WithRootErr(err)
	}
	if err := ctx.Validate(&req); err != nil {
		return errs.ErrBadRequest.WithMessage("Validation failed: " + err.Error())
	}

	user, err := h.updateProfileUC.Execute(ctx.Request().Context(), application.UpdateProfileRequest{
		UserID:      userID,
		DisplayName: req.DisplayName,
		AvatarURL:   req.AvatarURL,
	})
	if err != nil {
		return err
	}
	return response.WriteSuccess(ctx, http.StatusOK, user, "Profile updated successfully")
}

func (h *UserHandler) UpdateRole(ctx *echo.Context) error {
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		return errs.ErrBadRequest.WithMessage("Invalid user ID").WithRootErr(err)
	}

	var req updateRoleRequest
	if err := ctx.Bind(&req); err != nil {
		return errs.ErrBadRequest.WithMessage("Invalid request body").WithRootErr(err)
	}
	if err := ctx.Validate(&req); err != nil {
		return errs.ErrBadRequest.WithMessage("Validation failed: " + err.Error())
	}

	user, err := h.updateRoleUC.Execute(ctx.Request().Context(), application.UpdateRoleRequest{
		UserID:  id,
		NewRole: domain.AccountRole(req.Role),
	})
	if err != nil {
		return err
	}
	return response.WriteSuccess(ctx, http.StatusOK, user, "User role updated")
}

func (h *UserHandler) UpdateStatus(ctx *echo.Context) error {
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		return errs.ErrBadRequest.WithMessage("Invalid user ID").WithRootErr(err)
	}

	var req updateStatusRequest
	if err := ctx.Bind(&req); err != nil {
		return errs.ErrBadRequest.WithMessage("Invalid request body").WithRootErr(err)
	}
	if err := ctx.Validate(&req); err != nil {
		return errs.ErrBadRequest.WithMessage("Validation failed: " + err.Error())
	}

	user, err := h.updateStatusUC.Execute(ctx.Request().Context(), application.UpdateStatusRequest{
		UserID:    id,
		NewStatus: domain.AccountStatus(req.Status),
	})
	if err != nil {
		return err
	}
	return response.WriteSuccess(ctx, http.StatusOK, user, "User status updated")
}
