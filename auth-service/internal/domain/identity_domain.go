package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type EventType string

const (
	AggregateTypeIdentity = "IDENTITY"

	EventTypeIdentityCreated EventType = "IDENTITY_CREATED"
)

type Identity struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	Phone        string    `json:"phone"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type RefreshToken struct {
	ID         uuid.UUID
	IdentityID uuid.UUID
	TokenHash  string
	DeviceInfo string
	IPAddress  string
	UserAgent  string
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	LastUsedAt *time.Time
	CreatedAt  time.Time
}

type CreateIdentityWithOutbox struct {
	Identity      *Identity
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
}

type IdentityRepository interface {
	CreateWithOutbox(ctx context.Context, in CreateIdentityWithOutbox) (*Identity, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Identity, error)
	GetByEmail(ctx context.Context, email string) (*Identity, error)
	GetByPhone(ctx context.Context, phone string) (*Identity, error)
}

type CreateRefreshTokenInput struct {
	IdentityID uuid.UUID
	TokenHash  string
	DeviceInfo string
	IPAddress  string
	UserAgent  string
	ExpiresAt  time.Time
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, in CreateRefreshTokenInput) (*RefreshToken, error)
	GetByHash(ctx context.Context, hash string) (*RefreshToken, error)
	RevokeByHash(ctx context.Context, hash string) error
	RevokeAllByIdentity(ctx context.Context, identityID uuid.UUID) error
	TouchLastUsed(ctx context.Context, hash string) error
}
