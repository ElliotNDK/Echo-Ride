package application

import (
	"context"
	"echo-ride/pkg-library/errs"
	"echo-ride/services/user-service/internal/domain"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type UpsertUserRequest struct {
	UserID uuid.UUID
	Email  string
	Phone  string
	Role   domain.AccountRole
}

type UpsertUserUseCase interface {
	Execute(ctx context.Context, req UpsertUserRequest) (*domain.User, error)
}

type upsertUserUC struct {
	repo   domain.UserRepository
	logger *zap.Logger
}

func NewUpsertUserUseCase(repo domain.UserRepository, logger *zap.Logger) UpsertUserUseCase {
	return &upsertUserUC{repo: repo, logger: logger}
}

func (u *upsertUserUC) Execute(ctx context.Context, req UpsertUserRequest) (*domain.User, error) {
	if req.UserID == uuid.Nil {
		return nil, errs.ErrBadRequest.WithMessage("user_id is required")
	}
	if !req.Role.IsValid() {
		req.Role = domain.RoleRider
	}
	user, err := u.repo.Upsert(ctx, domain.UpsertUserInput{
		ID:    req.UserID,
		Email: req.Email,
		Phone: req.Phone,
		Role:  req.Role,
	})
	if err != nil {
		u.logger.Error("failed to upsert user", zap.Error(err), zap.String("user_id", req.UserID.String()))
		return nil, errs.ErrInternal.WithMessage("Failed to upsert user")
	}
	return user, nil
}
