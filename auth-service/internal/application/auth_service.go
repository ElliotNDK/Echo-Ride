package application

import (
	"echo-ride/services/auth-service/internal/domain"

	"go.uber.org/zap"
)

type authService struct {
	repo   domain.IdentityRepository
	logger *zap.Logger
}

func NewAuthService(repo domain.IdentityRepository, logger *zap.Logger) *authService {
	return &authService{
		repo:   repo,
		logger: logger,
	}
}
