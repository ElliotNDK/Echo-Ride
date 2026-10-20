package application

import (
	"context"
	"echo-ride/pkg-library/errs"
	"echo-ride/services/auth-service/internal/domain"
	"echo-ride/services/auth-service/internal/infrastructure/redis"
	"echo-ride/services/auth-service/pkg/jwt"
	"time"

	"go.uber.org/zap"
)

type LogoutRequest struct {
	RefreshToken string
	JTI          string
	AccessExp    time.Time
}

type LogoutUseCase interface {
	Execute(ctx context.Context, req LogoutRequest) error
}

type logoutUC struct {
	refreshRepo domain.RefreshTokenRepository
	blacklist   redis.Blacklist
	logger      *zap.Logger
}

func NewLogoutUseCase(refreshRepo domain.RefreshTokenRepository, blacklist redis.Blacklist, logger *zap.Logger) LogoutUseCase {
	return &logoutUC{refreshRepo: refreshRepo, blacklist: blacklist, logger: logger}
}

func (u *logoutUC) Execute(ctx context.Context, req LogoutRequest) error {
	if req.RefreshToken == "" {
		return errs.ErrBadRequest.WithMessage("refresh_token is required")
	}

	if err := u.refreshRepo.RevokeByHash(ctx, jwt.HashToken(req.RefreshToken)); err != nil {
		u.logger.Error("failed to revoke refresh token", zap.Error(err))
		return errs.ErrInternal.WithMessage("Failed to logout")
	}

	if req.JTI != "" {
		ttl := time.Until(req.AccessExp)
		if ttl > 0 {
			if err := u.blacklist.Add(ctx, req.JTI, ttl); err != nil {
				u.logger.Warn("failed to blacklist jti on logout", zap.Error(err))
			}
		}
	}

	return nil
}
