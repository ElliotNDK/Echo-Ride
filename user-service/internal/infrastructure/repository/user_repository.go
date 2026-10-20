package repository

import (
	"context"
	"echo-ride/services/user-service/internal/domain"
	"echo-ride/services/user-service/internal/infrastructure/db/dbgen"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepositoryImpl struct {
	db *pgxpool.Pool
	q  *dbgen.Queries
}

func NewUserRepository(db *pgxpool.Pool) domain.UserRepository {
	return &UserRepositoryImpl{
		db: db,
		q:  dbgen.New(db),
	}
}

func toPgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func toPgText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

func toPgTextPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func dbUserToDomain(u dbgen.TUser) *domain.User {
	return &domain.User{
		ID:          uuid.UUID(u.UserID.Bytes),
		Email:       u.UserEmail.String,
		Phone:       u.UserPhone.String,
		DisplayName: u.UserDisplayName.String,
		AvatarURL:   u.UserAvatarUrl.String,
		Role:        domain.AccountRole(u.UserRole),
		Status:      domain.AccountStatus(u.UserStatus),
		CreatedAt:   u.UserCreatedAt.Time,
		UpdatedAt:   u.UserUpdatedAt.Time,
	}
}

func (r *UserRepositoryImpl) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	row, err := r.q.GetUserByID(ctx, toPgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}
	return dbUserToDomain(row), nil
}

func (r *UserRepositoryImpl) Upsert(ctx context.Context, in domain.UpsertUserInput) (*domain.User, error) {
	if !in.Role.IsValid() {
		in.Role = domain.RoleRider
	}
	row, err := r.q.UpsertUser(ctx, dbgen.UpsertUserParams{
		UserID:    toPgUUID(in.ID),
		UserEmail: toPgText(in.Email),
		UserPhone: toPgText(in.Phone),
		UserRole:  dbgen.AccountRole(in.Role),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upsert user: %w", err)
	}
	return dbUserToDomain(row), nil
}

func (r *UserRepositoryImpl) UpdateProfile(ctx context.Context, id uuid.UUID, in domain.UpdateProfileInput) (*domain.User, error) {
	row, err := r.q.UpdateUserProfile(ctx, dbgen.UpdateUserProfileParams{
		UserID:          toPgUUID(id),
		UserDisplayName: toPgTextPtr(in.DisplayName),
		UserAvatarUrl:   toPgTextPtr(in.AvatarURL),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to update user profile: %w", err)
	}
	return dbUserToDomain(row), nil
}

func (r *UserRepositoryImpl) UpdateRoleWithOutbox(ctx context.Context, id uuid.UUID, role domain.AccountRole, evt domain.OutboxEvent) (*domain.User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := r.q.WithTx(tx)

	row, err := qtx.UpdateUserRole(ctx, dbgen.UpdateUserRoleParams{
		UserID:   toPgUUID(id),
		UserRole: dbgen.AccountRole(role),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to update user role: %w", err)
	}

	if _, err := qtx.CreateOutboxEvent(ctx, dbgen.CreateOutboxEventParams{
		EventAggregateType: evt.AggregateType,
		EventAggregateID:   evt.AggregateID,
		EventType:          evt.EventType,
		EventPayload:       evt.Payload,
	}); err != nil {
		return nil, fmt.Errorf("failed to create outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit tx: %w", err)
	}
	return dbUserToDomain(row), nil
}

func (r *UserRepositoryImpl) UpdateStatusWithOutbox(ctx context.Context, id uuid.UUID, status domain.AccountStatus, evt domain.OutboxEvent) (*domain.User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := r.q.WithTx(tx)

	row, err := qtx.UpdateUserStatus(ctx, dbgen.UpdateUserStatusParams{
		UserID:     toPgUUID(id),
		UserStatus: dbgen.AccountStatus(status),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to update user status: %w", err)
	}

	if _, err := qtx.CreateOutboxEvent(ctx, dbgen.CreateOutboxEventParams{
		EventAggregateType: evt.AggregateType,
		EventAggregateID:   evt.AggregateID,
		EventType:          evt.EventType,
		EventPayload:       evt.Payload,
	}); err != nil {
		return nil, fmt.Errorf("failed to create outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit tx: %w", err)
	}
	return dbUserToDomain(row), nil
}
