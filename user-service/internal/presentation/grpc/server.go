package grpc

import (
	"context"
	"echo-ride/pkg-library/errs"
	pb "echo-ride/pkg-library/grpc/user/v1"
	"echo-ride/services/user-service/internal/application"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type UserGrpcServer struct {
	pb.UnimplementedUserServiceServer
	getUserUC application.GetUserUseCase
	logger    *zap.Logger
	tracer    trace.Tracer
}

func NewUserGrpcServer(getUserUC application.GetUserUseCase, logger *zap.Logger) *UserGrpcServer {
	return &UserGrpcServer{
		getUserUC: getUserUC,
		logger:    logger,
		tracer:    otel.Tracer("user-grpc-server"),
	}
}

func (s *UserGrpcServer) GetUserAuthInfo(ctx context.Context, req *pb.GetUserAuthInfoRequest) (*pb.GetUserAuthInfoResponse, error) {
	ctx, span := s.tracer.Start(ctx, "GRPC GetUserAuthInfo")
	defer span.End()

	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, errs.ErrInvalidArgument.WithMessage("Invalid user_id").WithRootErr(err)
	}

	user, err := s.getUserUC.Execute(ctx, userID)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	return &pb.GetUserAuthInfoResponse{
		Role:   string(user.Role),
		Status: string(user.Status),
	}, nil
}
