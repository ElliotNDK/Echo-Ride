package redis

import (
	"context"
	"echo-ride/services/location-service/internal/domain"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	DriverLocationKey  = "driver_locations"
	DriverTimestampKey = "driver_timestamps"
)

type redisLocationRepo struct {
	client *redis.Client
}

func NewRedisLocationRepo(client *redis.Client) domain.ActiveDriverRepository {
	return &redisLocationRepo{client: client}
}

func (r *redisLocationRepo) UpsertActiveLocation(ctx context.Context, locations []domain.DriverLocation) error {
	if len(locations) == 0 {
		return nil
	}

	pipe := r.client.Pipeline()
	now := float64(time.Now().Unix())

	for _, loc := range locations {
		pipe.GeoAdd(ctx, DriverLocationKey, &redis.GeoLocation{
			Name:      loc.DriverID.String(),
			Longitude: loc.Lng,
			Latitude:  loc.Lat,
		})

		pipe.ZAdd(ctx, DriverTimestampKey, redis.Z{
			Score:  now,
			Member: loc.DriverID.String(),
		})
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (r *redisLocationRepo) RemoveStaleDrivers(ctx context.Context, olderThan time.Time) (int64, error) {
	staleScore := fmt.Sprintf("(%d", olderThan.Unix())

	staleDrivers, err := r.client.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     DriverTimestampKey,
		Start:   "-inf",
		Stop:    staleScore,
		ByScore: true,
	}).Result()

	if err != nil {
		return 0, err
	}

	if len(staleDrivers) == 0 {
		return 0, nil
	}

	pipe := r.client.Pipeline()

	interfaces := make([]interface{}, len(staleDrivers))
	for i, v := range staleDrivers {
		interfaces[i] = v
	}

	pipe.ZRem(ctx, DriverTimestampKey, interfaces...)
	pipe.ZRem(ctx, DriverLocationKey, interfaces...)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}

	return int64(len(staleDrivers)), nil
}

func (r *redisLocationRepo) FindNearestDrivers(ctx context.Context, lat, lng, radius float64, limit int) ([]domain.DriverLocation, error) {
	query := &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude:  lng,
			Latitude:   lat,
			Radius:     radius,
			RadiusUnit: "km",
			Sort:       "ASC",
			Count:      limit,
		},
		WithCoord: true, // Include coordinates of the drivers in the result
		WithDist:  true, // Include distance from the query point in the result
	}

	res, err := r.client.GeoSearchLocation(ctx, DriverLocationKey, query).Result()
	if err != nil {
		return nil, err
	}

	var drivers []domain.DriverLocation
	for _, loc := range res {
		parsedDriverID, err := uuid.Parse(loc.Name)
		if err != nil {
			continue
		}

		drivers = append(drivers, domain.DriverLocation{
			DriverID:   parsedDriverID,
			Lat:        loc.Latitude,
			Lng:        loc.Longitude,
			DistanceKm: loc.Dist,
		})
	}

	return drivers, nil
}

func (r *redisLocationRepo) FindDriversInRadius(ctx context.Context, lat, lng, radius float64, limit int) ([]domain.DriverLocation, error) {
	query := &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude:  lng,
			Latitude:   lat,
			Radius:     radius,
			RadiusUnit: "km",
			Count:      limit,
			Sort:       "ASC",
		},
		WithCoord: true, // Include coordinates of the drivers in the result
		WithDist:  true, // Include distance from the query point in the result
	}

	locations, err := r.client.GeoSearchLocation(ctx, DriverLocationKey, query).Result()
	if err != nil {
		if err == redis.Nil {
			return []domain.DriverLocation{}, nil
		}
		return nil, fmt.Errorf("failed to find drivers in radius: %w", err)
	}

	var results []domain.DriverLocation
	for _, loc := range locations {
		driverID, err := uuid.Parse(loc.Name)
		if err != nil {
			continue
		}

		results = append(results, domain.DriverLocation{
			DriverID:   driverID,
			Lat:        loc.Latitude,
			Lng:        loc.Longitude,
			DistanceKm: loc.Dist,
		})
	}

	return results, nil
}
