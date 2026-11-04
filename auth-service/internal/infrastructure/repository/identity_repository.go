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

type IdentityRepositoryImpl struct {
	db *pgxpool.Pool
	q  *dbgen.Queries
}

func NewIdentityRepository(db *pgxpool.Pool) domain.IdentityRepository {
	return &IdentityRepositoryImpl{
		db: db,
		q:  dbgen.New(db),
	}
}

func toPgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func dbIdentityToDomain(dbId dbgen.TIdentity) *domain.Identity {
	return &domain.Identity{
		ID:           uuid.UUID(dbId.IdentityID.Bytes),
		Email:        dbId.IdentityEmail,
		Phone:        dbId.IdentityPhone,
		PasswordHash: dbId.IdentityPasswordHash,
		CreatedAt:    dbId.IdentityCreatedAt.Time,
		UpdatedAt:    dbId.IdentityUpdatedAt.Time,
	}
}

func (i *IdentityRepositoryImpl) CreateWithOutbox(ctx context.Context, in domain.CreateIdentityWithOutbox) (*domain.Identity, error) {
	tx, err := i.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := i.q.WithTx(tx)

	id := in.Identity.ID
	if id == uuid.Nil {
		id = uuid.New()
	}

	dbId, err := qtx.CreateIdentity(ctx, dbgen.CreateIdentityParams{
		IdentityID:           toPgUUID(id),
		IdentityEmail:        in.Identity.Email,
		IdentityPhone:        in.Identity.Phone,
		IdentityPasswordHash: in.Identity.PasswordHash,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create identity: %w", err)
	}

	if _, err := qtx.CreateOutboxEvent(ctx, dbgen.CreateOutboxEventParams{
		EventAggregateType: in.AggregateType,
		EventAggregateID:   in.AggregateID,
		EventType:          in.EventType,
		EventPayload:       in.Payload,
	}); err != nil {
		return nil, fmt.Errorf("failed to create outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit tx: %w", err)
	}

	return dbIdentityToDomain(dbId), nil
}

func (i *IdentityRepositoryImpl) GetByID(ctx context.Context, id uuid.UUID) (*domain.Identity, error) {
	dbId, err := i.q.GetIdentityByID(ctx, toPgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get identity by id: %w", err)
	}
	return dbIdentityToDomain(dbId), nil
}

func (i *IdentityRepositoryImpl) GetByEmail(ctx context.Context, email string) (*domain.Identity, error) {
	dbId, err := i.q.GetIdentityByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get identity by email: %w", err)
	}
	return dbIdentityToDomain(dbId), nil
}

func (i *IdentityRepositoryImpl) GetByPhone(ctx context.Context, phone string) (*domain.Identity, error) {
	dbId, err := i.q.GetIdentityByPhone(ctx, phone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get identity by phone: %w", err)
	}
	return dbIdentityToDomain(dbId), nil
}
