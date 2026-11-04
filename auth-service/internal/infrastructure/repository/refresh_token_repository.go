package repository

import (
	"context"
	"echo-ride/services/auth-service/internal/domain"
	"echo-ride/services/auth-service/internal/infrastructure/db/dbgen"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RefreshTokenRepositoryImpl struct {
	q *dbgen.Queries
}

func NewRefreshTokenRepository(db *pgxpool.Pool) domain.RefreshTokenRepository {
	return &RefreshTokenRepositoryImpl{q: dbgen.New(db)}
}

func toPgText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

func dbRefreshToDomain(r dbgen.TRefreshToken) *domain.RefreshToken {
	rt := &domain.RefreshToken{
		ID:         uuid.UUID(r.RefreshTokenID.Bytes),
		IdentityID: uuid.UUID(r.RefreshTokenIdentityID.Bytes),
		TokenHash:  r.RefreshTokenHash,
		DeviceInfo: r.RefreshTokenDeviceInfo.String,
		IPAddress:  r.RefreshTokenIpAddress.String,
		UserAgent:  r.RefreshTokenUserAgent.String,
		ExpiresAt:  r.RefreshTokenExpiresAt.Time,
		CreatedAt:  r.RefreshTokenCreatedAt.Time,
	}
	if r.RefreshTokenRevokedAt.Valid {
		t := r.RefreshTokenRevokedAt.Time
		rt.RevokedAt = &t
	}
	if r.RefreshTokenLastUsedAt.Valid {
		t := r.RefreshTokenLastUsedAt.Time
		rt.LastUsedAt = &t
	}
	return rt
}

func (r *RefreshTokenRepositoryImpl) Create(ctx context.Context, in domain.CreateRefreshTokenInput) (*domain.RefreshToken, error) {
	row, err := r.q.CreateRefreshToken(ctx, dbgen.CreateRefreshTokenParams{
		RefreshTokenIdentityID: toPgUUID(in.IdentityID),
		RefreshTokenHash:       in.TokenHash,
		RefreshTokenDeviceInfo: toPgText(in.DeviceInfo),
		RefreshTokenIpAddress:  toPgText(in.IPAddress),
		RefreshTokenUserAgent:  toPgText(in.UserAgent),
		RefreshTokenExpiresAt:  pgtype.Timestamptz{Time: in.ExpiresAt, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh token: %w", err)
	}
	return dbRefreshToDomain(row), nil
}

func (r *RefreshTokenRepositoryImpl) GetByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	row, err := r.q.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}
	return dbRefreshToDomain(row), nil
}

func (r *RefreshTokenRepositoryImpl) RevokeByHash(ctx context.Context, hash string) error {
	if err := r.q.RevokeRefreshTokenByHash(ctx, hash); err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepositoryImpl) RevokeAllByIdentity(ctx context.Context, identityID uuid.UUID) error {
	if err := r.q.RevokeAllRefreshTokensByIdentity(ctx, toPgUUID(identityID)); err != nil {
		return fmt.Errorf("failed to revoke all refresh tokens: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepositoryImpl) TouchLastUsed(ctx context.Context, hash string) error {
	if err := r.q.TouchRefreshTokenLastUsed(ctx, hash); err != nil {
		return fmt.Errorf("failed to touch refresh token: %w", err)
	}
	return nil
}
