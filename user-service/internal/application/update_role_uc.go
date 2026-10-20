package application

import (
	"context"
	"echo-ride/pkg-library/errs"
	"echo-ride/services/user-service/internal/domain"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type UpdateRoleRequest struct {
	UserID  uuid.UUID
	NewRole domain.AccountRole
}

type UpdateRoleUseCase interface {
	Execute(ctx context.Context, req UpdateRoleRequest) (*domain.User, error)
}

type updateRoleUC struct {
	repo   domain.UserRepository
	logger *zap.Logger
}

func NewUpdateRoleUseCase(repo domain.UserRepository, logger *zap.Logger) UpdateRoleUseCase {
	return &updateRoleUC{repo: repo, logger: logger}
}

type RoleChangedPayload struct {
	UserID    string `json:"user_id"`
	NewRole   string `json:"new_role"`
	ChangedAt string `json:"changed_at"`
}

func (u *updateRoleUC) Execute(ctx context.Context, req UpdateRoleRequest) (*domain.User, error) {
	if !req.NewRole.IsValid() {
		return nil, errs.ErrBadRequest.WithMessage("Invalid role")
	}

	payload, err := json.Marshal(RoleChangedPayload{
		UserID:    req.UserID.String(),
		NewRole:   string(req.NewRole),
		ChangedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, errs.ErrInternal.WithMessage("Failed to marshal event payload").WithRootErr(err)
	}

	user, err := u.repo.UpdateRoleWithOutbox(ctx, req.UserID, req.NewRole, domain.OutboxEvent{
		AggregateType: domain.AggregateTypeUser,
		AggregateID:   req.UserID.String(),
		EventType:     string(domain.EventTypeRoleChanged),
		Payload:       payload,
	})
	if err != nil {
		u.logger.Error("failed to update role", zap.Error(err))
		return nil, errs.ErrInternal.WithMessage("Failed to update role")
	}
	if user == nil {
		return nil, errs.ErrNotFound.WithMessage("User not found")
	}
	return user, nil
}
