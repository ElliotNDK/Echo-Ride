package domain

import "context"

type MatchingEventPublisher interface {
	PublishDriverMatched(ctx context.Context, rideID, driverID string) error
	PublishMatchingFailed(ctx context.Context, rideID string) error
}
