package application

import (
	"context"
	"echo-ride/pkg-library/errs"
	"echo-ride/services/auth-service/internal/domain"
	grpcclient "echo-ride/services/auth-service/internal/infrastructure/grpc-client"
	"echo-ride/services/auth-service/pkg/hash"
	"echo-ride/services/auth-service/pkg/jwt"
	"time"

	"go.uber.org/zap"
)

type LoginRequest struct {
	Phone      string `json:"phone"`
	Password   string `json:"password"`
	DeviceInfo string `json:"deviceInfo,omitempty"`
	IPAddress  string `json:"-"`
	UserAgent  string `json:"-"`
}

type LoginResponse struct {
	AccessToken           string    `json:"accessToken"`
	AccessTokenExpiresAt  time.Time `json:"accessTokenExpiresAt"`
	RefreshToken          string    `json:"refreshToken"`
	RefreshTokenExpiresAt time.Time `json:"refreshTokenExpiresAt"`
}

type LoginUseCase interface {
	Execute(ctx context.Context, req LoginRequest) (*LoginResponse, error)
}

type loginUC struct {
	identityRepo    domain.IdentityRepository
	refreshRepo     domain.RefreshTokenRepository
	userClient      grpcclient.UserServiceClient
	tokenMaker      *jwt.TokenMaker
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
	logger          *zap.Logger
}

type LoginUCDeps struct {
	IdentityRepo    domain.IdentityRepository
	RefreshRepo     domain.RefreshTokenRepository
	UserClient      grpcclient.UserServiceClient
	TokenMaker      *jwt.TokenMaker
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Logger          *zap.Logger
}

func NewLoginUseCase(deps LoginUCDeps) LoginUseCase {
	return &loginUC{
		identityRepo:    deps.IdentityRepo,
		refreshRepo:     deps.RefreshRepo,
		userClient:      deps.UserClient,
		tokenMaker:      deps.TokenMaker,
		accessTokenTTL:  deps.AccessTokenTTL,
		refreshTokenTTL: deps.RefreshTokenTTL,
		logger:          deps.Logger,
	}
}

func (l *loginUC) Execute(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	identity, err := l.identityRepo.GetByPhone(ctx, req.Phone)
	if err != nil {
		l.logger.Error("failed to get identity by phone", zap.Error(err))
		return nil, errs.ErrInternal.WithMessage("Database error")
	}
	if identity == nil {
		return nil, errs.ErrUnauthorized.WithMessage("Invalid phone number or password")
	}

	if err := hash.CheckPasswordHash(req.Password, identity.PasswordHash); err != nil {
		return nil, errs.ErrUnauthorized.WithMessage("Invalid phone number or password")
	}

	authInfo, err := l.userClient.GetUserAuthInfo(ctx, identity.ID)
	if err != nil {
		l.logger.Error("failed to get user auth info from user-service", zap.Error(err), zap.String("user_id", identity.ID.String()))
		return nil, errs.ErrServiceCallFailed.WithMessage("Unable to verify user role/status")
	}
	if authInfo.Status != "ACTIVE" {
		return nil, errs.ErrForbidden.WithMessage("Account is " + authInfo.Status)
	}

	access, err := l.tokenMaker.GenerateAccessToken(identity.ID, authInfo.Role, authInfo.Status, l.accessTokenTTL)
	if err != nil {
		l.logger.Error("failed to generate access token", zap.Error(err))
		return nil, errs.ErrInternal.WithMessage("Failed to generate access token")
	}

	refresh, err := l.tokenMaker.GenerateRefreshToken(l.refreshTokenTTL)
	if err != nil {
		l.logger.Error("failed to generate refresh token", zap.Error(err))
		return nil, errs.ErrInternal.WithMessage("Failed to generate refresh token")
	}

	if _, err := l.refreshRepo.Create(ctx, domain.CreateRefreshTokenInput{
		IdentityID: identity.ID,
		TokenHash:  refresh.Hash,
		DeviceInfo: req.DeviceInfo,
		IPAddress:  req.IPAddress,
		UserAgent:  req.UserAgent,
		ExpiresAt:  refresh.ExpiresAt,
	}); err != nil {
		l.logger.Error("failed to persist refresh token", zap.Error(err))
		return nil, errs.ErrInternal.WithMessage("Failed to persist refresh token")
	}

	return &LoginResponse{
		AccessToken:           access.Token,
		AccessTokenExpiresAt:  access.ExpiresAt,
		RefreshToken:          refresh.Token,
		RefreshTokenExpiresAt: refresh.ExpiresAt,
	}, nil
}
