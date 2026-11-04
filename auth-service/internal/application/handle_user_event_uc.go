package application

import (
	"context"
	"echo-ride/services/auth-service/internal/domain"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type HandleUserEventUseCase interface {
	HandleStatusChanged(ctx context.Context, userID uuid.UUID, newStatus string) error
	HandleRoleChanged(ctx context.Context, userID uuid.UUID, newRole string) error
}

type handleUserEventUC struct {
	refreshRepo domain.RefreshTokenRepository
	logger      *zap.Logger
}

func NewHandleUserEventUseCase(refreshRepo domain.RefreshTokenRepository, logger *zap.Logger) HandleUserEventUseCase {
	return &handleUserEventUC{refreshRepo: refreshRepo, logger: logger}
}

func (u *handleUserEventUC) HandleStatusChanged(ctx context.Context, userID uuid.UUID, newStatus string) error {
	if newStatus == "ACTIVE" {
		return nil
	}
	if err := u.refreshRepo.RevokeAllByIdentity(ctx, userID); err != nil {
		u.logger.Error("failed to revoke refresh tokens on status change",
			zap.Error(err), zap.String("user_id", userID.String()), zap.String("new_status", newStatus))
		return err
	}
	u.logger.Info("Revoked all refresh tokens due to status change",
		zap.String("user_id", userID.String()), zap.String("new_status", newStatus))
	return nil
}

func (u *handleUserEventUC) HandleRoleChanged(ctx context.Context, userID uuid.UUID, newRole string) error {
	if err := u.refreshRepo.RevokeAllByIdentity(ctx, userID); err != nil {
		u.logger.Error("failed to revoke refresh tokens on role change",
			zap.Error(err), zap.String("user_id", userID.String()), zap.String("new_role", newRole))
		return err
	}
	u.logger.Info("Revoked all refresh tokens due to role change",
		zap.String("user_id", userID.String()), zap.String("new_role", newRole))
	return nil
}
