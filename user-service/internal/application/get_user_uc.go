package application

import (
	"context"
	"echo-ride/pkg-library/errs"
	"echo-ride/services/user-service/internal/domain"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type GetUserUseCase interface {
	Execute(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

type getUserUC struct {
	repo   domain.UserRepository
	logger *zap.Logger
}

func NewGetUserUseCase(repo domain.UserRepository, logger *zap.Logger) GetUserUseCase {
	return &getUserUC{repo: repo, logger: logger}
}

func (u *getUserUC) Execute(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	user, err := u.repo.GetByID(ctx, id)
	if err != nil {
		u.logger.Error("failed to get user", zap.Error(err), zap.String("user_id", id.String()))
		return nil, errs.ErrInternal.WithMessage("Failed to get user")
	}
	if user == nil {
		return nil, errs.ErrNotFound.WithMessage("User not found")
	}
	return user, nil
}
