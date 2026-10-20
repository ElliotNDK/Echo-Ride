package application

import (
	"context"
	"echo-ride/pkg-library/errs"
	"echo-ride/services/auth-service/internal/domain"
	"echo-ride/services/auth-service/pkg/hash"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type RegisterRequest struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type RegisterResponse struct {
	IdentityID uuid.UUID `json:"identityId"`
	Email      string    `json:"email"`
	Phone      string    `json:"phone"`
}

type RegisterUseCase interface {
	Execute(ctx context.Context, req RegisterRequest) (*RegisterResponse, error)
}

type registerUC struct {
	repo   domain.IdentityRepository
	logger *zap.Logger
}

func NewRegisterUseCase(repo domain.IdentityRepository, logger *zap.Logger) RegisterUseCase {
	return &registerUC{repo: repo, logger: logger}
}

type IdentityCreatedPayload struct {
	IdentityID string `json:"identity_id"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	Role       string `json:"role"`
	CreatedAt  string `json:"created_at"`
}

func (r *registerUC) Execute(ctx context.Context, req RegisterRequest) (*RegisterResponse, error) {
	if existing, _ := r.repo.GetByPhone(ctx, req.Phone); existing != nil {
		return nil, errs.ErrConflict.WithMessage("Phone number already exists")
	}
	if existing, _ := r.repo.GetByEmail(ctx, req.Email); existing != nil {
		return nil, errs.ErrConflict.WithMessage("Email already exists")
	}

	hashedPassword, err := hash.HashPassword(req.Password)
	if err != nil {
		r.logger.Error("failed to hash password", zap.Error(err))
		return nil, errs.ErrInternal.WithMessage("Failed to hash password")
	}

	identityID := uuid.New()
	role := req.Role
	if role == "" {
		role = "RIDER"
	}

	payload, err := json.Marshal(IdentityCreatedPayload{
		IdentityID: identityID.String(),
		Email:      req.Email,
		Phone:      req.Phone,
		Role:       role,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, errs.ErrInternal.WithMessage("Failed to marshal event payload").WithRootErr(err)
	}

	created, err := r.repo.CreateWithOutbox(ctx, domain.CreateIdentityWithOutbox{
		Identity: &domain.Identity{
			ID:           identityID,
			Email:        req.Email,
			Phone:        req.Phone,
			PasswordHash: hashedPassword,
		},
		AggregateType: domain.AggregateTypeIdentity,
		AggregateID:   identityID.String(),
		EventType:     string(domain.EventTypeIdentityCreated),
		Payload:       payload,
	})
	if err != nil {
		r.logger.Error("failed to create identity", zap.Error(err))
		return nil, errs.ErrInternal.WithMessage("Failed to create identity")
	}

	return &RegisterResponse{
		IdentityID: created.ID,
		Email:      created.Email,
		Phone:      created.Phone,
	}, nil
}
