package application

import (
	"context"
	"echo-ride/pkg-library/errs"
	"echo-ride/services/user-service/internal/domain"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type UpdateProfileRequest struct {
	UserID      uuid.UUID
	DisplayName *string
	AvatarURL   *string
}

type UpdateProfileUseCase interface {
	Execute(ctx context.Context, req UpdateProfileRequest) (*domain.User, error)
}

type updateProfileUC struct {
	repo   domain.UserRepository
	logger *zap.Logger
}

func NewUpdateProfileUseCase(repo domain.UserRepository, logger *zap.Logger) UpdateProfileUseCase {
	return &updateProfileUC{repo: repo, logger: logger}
}

func (u *updateProfileUC) Execute(ctx context.Context, req UpdateProfileRequest) (*domain.User, error) {
	if req.DisplayName == nil && req.AvatarURL == nil {
		return nil, errs.ErrBadRequest.WithMessage("Nothing to update")
	}
	user, err := u.repo.UpdateProfile(ctx, req.UserID, domain.UpdateProfileInput{
		DisplayName: req.DisplayName,
		AvatarURL:   req.AvatarURL,
	})
	if err != nil {
		u.logger.Error("failed to update profile", zap.Error(err))
		return nil, errs.ErrInternal.WithMessage("Failed to update profile")
	}
	if user == nil {
		return nil, errs.ErrNotFound.WithMessage("User not found")
	}
	return user, nil
}
