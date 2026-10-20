package grpc_client

import (
	"context"
	"echo-ride/pkg-library/errs"
	pb "echo-ride/pkg-library/grpc/location/v1"
	"echo-ride/services/matching-service/internal/domain"
	"errors"
	"time"

	"github.com/sony/gobreaker"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	circuitBreakerName = "LocationService-gRPC"
)

type locationGrpcClient struct {
	client         pb.LocationServiceClient
	circuitBreaker *gobreaker.CircuitBreaker
	logger         *zap.Logger
}

func NewLocationGrpcClient(targetUrl string, logger *zap.Logger) (domain.LocationGateway, error) {
	conn, err := grpc.NewClient(targetUrl, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		return nil, errs.ErrServiceConnectFailed.WithMessage("Failed to connect to Location Service").WithRootErr(err)
	}

	cbSettings := gobreaker.Settings{
		Name:        circuitBreakerName,
		MaxRequests: 5,
		Interval:    5 * time.Minute,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 10 && failureRatio >= 0.5
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Warn("LocationService circuit breaker state changed", zap.String("from", from.String()), zap.String("to", to.String()))
		},
	}

	client := pb.NewLocationServiceClient(conn)
	return &locationGrpcClient{
		client:         client,
		circuitBreaker: gobreaker.NewCircuitBreaker(cbSettings),
		logger:         logger,
	}, nil
}

func (l *locationGrpcClient) GetNearestDrivers(ctx context.Context, rideID string, lat, lng, radiusKm float64, limit int) ([]domain.CandidateDriver, error) {
	result, err := l.circuitBreaker.Execute(func() (interface{}, error) {
		ctxTimeout, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		req := &pb.FindNearestDriversRequest{
			RideId:    rideID,
			PickupLat: lat,
			PickupLng: lng,
			RadiusKm:  radiusKm,
			Limit:     int32(limit),
		}

		res, err := l.client.FindNearestDrivers(ctxTimeout, req)
		if err != nil {
			return nil, errs.ErrServiceCallFailed.WithMessage("Failed to call Location Service").WithRootErr(err)
		}

		return res, nil

	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			l.logger.Warn("LocationService circuit breaker is open, returning fallback response")
		} else {
			l.logger.Error("gRPC call to LocationService failed", zap.Error(err))
		}
		l.logger.Error("gRPC call to LocationService failed", zap.Error(err))
		return nil, err
	}

	res, ok := result.(*pb.FindNearestDriversResponse)
	if !ok {
		l.logger.Error("Type assertion failed! Unexpected type returned from Circuit Breaker")
		return nil, errs.ErrServiceCallFailed.WithMessage("Type assertion failed")
	}

	var candidates []domain.CandidateDriver
	for _, d := range res.Drivers {
		candidates = append(candidates, domain.CandidateDriver{
			DriverID:   d.DriverId,
			Lat:        d.Lat,
			Lng:        d.Lng,
			DistanceKm: d.DistanceKm,
			Score:      float64(d.EtaSeconds),
		})
	}

	return candidates, nil
}
