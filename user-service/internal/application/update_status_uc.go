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

type UpdateStatusRequest struct {
	UserID    uuid.UUID
	NewStatus domain.AccountStatus
}

type UpdateStatusUseCase interface {
	Execute(ctx context.Context, req UpdateStatusRequest) (*domain.User, error)
}

type updateStatusUC struct {
	repo   domain.UserRepository
	logger *zap.Logger
}

func NewUpdateStatusUseCase(repo domain.UserRepository, logger *zap.Logger) UpdateStatusUseCase {
	return &updateStatusUC{repo: repo, logger: logger}
}

type StatusChangedPayload struct {
	UserID    string `json:"user_id"`
	NewStatus string `json:"new_status"`
	ChangedAt string `json:"changed_at"`
}

func (u *updateStatusUC) Execute(ctx context.Context, req UpdateStatusRequest) (*domain.User, error) {
	if !req.NewStatus.IsValid() {
		return nil, errs.ErrBadRequest.WithMessage("Invalid status")
	}

	payload, err := json.Marshal(StatusChangedPayload{
		UserID:    req.UserID.String(),
		NewStatus: string(req.NewStatus),
		ChangedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, errs.ErrInternal.WithMessage("Failed to marshal event payload").WithRootErr(err)
	}

	user, err := u.repo.UpdateStatusWithOutbox(ctx, req.UserID, req.NewStatus, domain.OutboxEvent{
		AggregateType: domain.AggregateTypeUser,
		AggregateID:   req.UserID.String(),
		EventType:     string(domain.EventTypeStatusChanged),
		Payload:       payload,
	})
	if err != nil {
		u.logger.Error("failed to update status", zap.Error(err))
		return nil, errs.ErrInternal.WithMessage("Failed to update status")
	}
	if user == nil {
		return nil, errs.ErrNotFound.WithMessage("User not found")
	}
	return user, nil
}
