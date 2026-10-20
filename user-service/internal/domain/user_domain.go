package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type AccountRole string
type AccountStatus string
type EventType string

const (
	RoleRider  AccountRole = "RIDER"
	RoleDriver AccountRole = "DRIVER"
	RoleAdmin  AccountRole = "ADMIN"

	StatusActive    AccountStatus = "ACTIVE"
	StatusSuspended AccountStatus = "SUSPENDED"
	StatusBanned    AccountStatus = "BANNED"
)

const (
	AggregateTypeUser = "USER"

	EventTypeRoleChanged     EventType = "ROLE_CHANGED"
	EventTypeStatusChanged   EventType = "STATUS_CHANGED"
	EventTypeProfileUpdated  EventType = "PROFILE_UPDATED"
	EventTypeIdentityCreated EventType = "IDENTITY_CREATED" // consumed from auth-service
)

func (r AccountRole) IsValid() bool {
	switch r {
	case RoleRider, RoleDriver, RoleAdmin:
		return true
	}
	return false
}

func (s AccountStatus) IsValid() bool {
	switch s {
	case StatusActive, StatusSuspended, StatusBanned:
		return true
	}
	return false
}

type User struct {
	ID          uuid.UUID     `json:"id"`
	Email       string        `json:"email,omitempty"`
	Phone       string        `json:"phone,omitempty"`
	DisplayName string        `json:"displayName,omitempty"`
	AvatarURL   string        `json:"avatarUrl,omitempty"`
	Role        AccountRole   `json:"role"`
	Status      AccountStatus `json:"status"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
}

type UpsertUserInput struct {
	ID    uuid.UUID
	Email string
	Phone string
	Role  AccountRole
}

type UpdateProfileInput struct {
	DisplayName *string
	AvatarURL   *string
}

type OutboxEvent struct {
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
}

type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	Upsert(ctx context.Context, in UpsertUserInput) (*User, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, in UpdateProfileInput) (*User, error)
	UpdateRoleWithOutbox(ctx context.Context, id uuid.UUID, role AccountRole, evt OutboxEvent) (*User, error)
	UpdateStatusWithOutbox(ctx context.Context, id uuid.UUID, status AccountStatus, evt OutboxEvent) (*User, error)
}
