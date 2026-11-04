package grpcclient

import (
	"context"
	"echo-ride/pkg-library/errs"
	pb "echo-ride/pkg-library/grpc/user/v1"
	"fmt"

	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type UserAuthInfo struct {
	Role   string
	Status string
}

type UserServiceClient interface {
	GetUserAuthInfo(ctx context.Context, userID uuid.UUID) (*UserAuthInfo, error)
	Close() error
}

type userServiceClient struct {
	conn *grpc.ClientConn
	c    pb.UserServiceClient
}

func NewUserServiceClient(addr string) (UserServiceClient, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial user-service grpc at %s: %w", addr, err)
	}
	return &userServiceClient{
		conn: conn,
		c:    pb.NewUserServiceClient(conn),
	}, nil
}

func (u *userServiceClient) GetUserAuthInfo(ctx context.Context, userID uuid.UUID) (*UserAuthInfo, error) {
	resp, err := u.c.GetUserAuthInfo(ctx, &pb.GetUserAuthInfoRequest{UserId: userID.String()})
	if err != nil {
		return nil, errs.ErrServiceCallFailed.WithMessage("user-service GetUserAuthInfo failed").WithRootErr(err)
	}
	return &UserAuthInfo{Role: resp.GetRole(), Status: resp.GetStatus()}, nil
}

func (u *userServiceClient) Close() error {
	return u.conn.Close()
}
