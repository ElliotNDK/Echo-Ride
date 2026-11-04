package application

import (
	"context"
	"echo-ride/pkg-library/errs"
	"echo-ride/services/auth-service/internal/domain"
	grpcclient "echo-ride/services/auth-service/internal/infrastructure/grpc-client"
	"echo-ride/services/auth-service/pkg/jwt"
	"time"

	"go.uber.org/zap"
)

type RefreshRequest struct {
	RefreshToken string
	DeviceInfo   string
	IPAddress    string
	UserAgent    string
}

type RefreshResponse struct {
	AccessToken           string    `json:"accessToken"`
	AccessTokenExpiresAt  time.Time `json:"accessTokenExpiresAt"`
	RefreshToken          string    `json:"refreshToken"`
	RefreshTokenExpiresAt time.Time `json:"refreshTokenExpiresAt"`
}

type RefreshUseCase interface {
	Execute(ctx context.Context, req RefreshRequest) (*RefreshResponse, error)
}

type refreshUC struct {
	refreshRepo     domain.RefreshTokenRepository
	userClient      grpcclient.UserServiceClient
	tokenMaker      *jwt.TokenMaker
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
	logger          *zap.Logger
}

type RefreshUCDeps struct {
	RefreshRepo     domain.RefreshTokenRepository
	UserClient      grpcclient.UserServiceClient
	TokenMaker      *jwt.TokenMaker
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Logger          *zap.Logger
}

func NewRefreshUseCase(deps RefreshUCDeps) RefreshUseCase {
	return &refreshUC{
		refreshRepo:     deps.RefreshRepo,
		userClient:      deps.UserClient,
		tokenMaker:      deps.TokenMaker,
		accessTokenTTL:  deps.AccessTokenTTL,
		refreshTokenTTL: deps.RefreshTokenTTL,
		logger:          deps.Logger,
	}
}

func (u *refreshUC) Execute(ctx context.Context, req RefreshRequest) (*RefreshResponse, error) {
	if req.RefreshToken == "" {
		return nil, errs.ErrBadRequest.WithMessage("refresh_token is required")
	}

	hash := jwt.HashToken(req.RefreshToken)
	stored, err := u.refreshRepo.GetByHash(ctx, hash)
	if err != nil {
		u.logger.Error("failed to fetch refresh token", zap.Error(err))
		return nil, errs.ErrInternal.WithMessage("Database error")
	}
	if stored == nil || stored.RevokedAt != nil || time.Now().After(stored.ExpiresAt) {
		// Reuse detection: if token already revoked, revoke ALL tokens of this user as a safety measure.
		if stored != nil && stored.RevokedAt != nil {
			_ = u.refreshRepo.RevokeAllByIdentity(ctx, stored.IdentityID)
		}
		return nil, errs.ErrUnauthorized.WithMessage("Invalid or expired refresh token")
	}

	authInfo, err := u.userClient.GetUserAuthInfo(ctx, stored.IdentityID)
	if err != nil {
		return nil, errs.ErrServiceCallFailed.WithMessage("Unable to verify user")
	}
	if authInfo.Status != "ACTIVE" {
		_ = u.refreshRepo.RevokeAllByIdentity(ctx, stored.IdentityID)
		return nil, errs.ErrForbidden.WithMessage("Account is " + authInfo.Status)
	}

	// Rotate: revoke old, issue new
	if err := u.refreshRepo.RevokeByHash(ctx, hash); err != nil {
		u.logger.Error("failed to revoke old refresh token", zap.Error(err))
		return nil, errs.ErrInternal.WithMessage("Failed to rotate refresh token")
	}

	access, err := u.tokenMaker.GenerateAccessToken(stored.IdentityID, authInfo.Role, authInfo.Status, u.accessTokenTTL)
	if err != nil {
		return nil, errs.ErrInternal.WithMessage("Failed to generate access token").WithRootErr(err)
	}

	newRefresh, err := u.tokenMaker.GenerateRefreshToken(u.refreshTokenTTL)
	if err != nil {
		return nil, errs.ErrInternal.WithMessage("Failed to generate refresh token").WithRootErr(err)
	}

	if _, err := u.refreshRepo.Create(ctx, domain.CreateRefreshTokenInput{
		IdentityID: stored.IdentityID,
		TokenHash:  newRefresh.Hash,
		DeviceInfo: req.DeviceInfo,
		IPAddress:  req.IPAddress,
		UserAgent:  req.UserAgent,
		ExpiresAt:  newRefresh.ExpiresAt,
	}); err != nil {
		return nil, errs.ErrInternal.WithMessage("Failed to persist new refresh token").WithRootErr(err)
	}

	return &RefreshResponse{
		AccessToken:           access.Token,
		AccessTokenExpiresAt:  access.ExpiresAt,
		RefreshToken:          newRefresh.Token,
		RefreshTokenExpiresAt: newRefresh.ExpiresAt,
	}, nil
}
