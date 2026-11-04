package redis

import (
	"context"
	"echo-ride/services/matching-service/internal/domain"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	StateKeyPrefix               = "matching:state:"
	TimeoutZSetKey               = "matching:timeouts"
	MatchingIdempotencyKeyPrefix = "matching:idempotency:"
)

type redisDispatchRepo struct {
	client *redis.Client
}

func NewRedisDispatchRepo(client *redis.Client) domain.DispatchRepository {
	return &redisDispatchRepo{
		client: client,
	}
}

func (r *redisDispatchRepo) SaveState(ctx context.Context, state domain.RideDispatchState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("%s%s", StateKeyPrefix, state.RideID)

	return r.client.Set(ctx, key, data, 1*time.Hour).Err()
}

func (r *redisDispatchRepo) GetState(ctx context.Context, rideID string) (*domain.RideDispatchState, error) {
	key := fmt.Sprintf("%s%s", StateKeyPrefix, rideID)
	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil // Not found
		}
		return nil, err
	}

	var state domain.RideDispatchState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

func (r *redisDispatchRepo) DeleteState(ctx context.Context, rideID string) error {
	key := fmt.Sprintf("%s%s", StateKeyPrefix, rideID)
	return r.client.Del(ctx, key).Err()
}

func (r *redisDispatchRepo) SetTimeout(ctx context.Context, rideID string, expireAt int64) error {
	return r.client.ZAdd(ctx, TimeoutZSetKey, redis.Z{
		Score:  float64(expireAt),
		Member: rideID,
	}).Err()
}

func (r *redisDispatchRepo) GetExpiredRides(ctx context.Context, now int64, limit int) ([]string, error) {
	opt := &redis.ZRangeBy{
		Min:   "-inf",
		Max:   fmt.Sprintf("%d", now),
		Count: int64(limit),
	}

	rides, err := r.client.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     TimeoutZSetKey,
		Start:   opt.Min,
		Stop:    opt.Max,
		ByScore: true,
		Offset:  0,
		Count:   opt.Count,
	}).Result()
	if err != nil {
		return nil, err
	}

	return rides, nil
}

func (r *redisDispatchRepo) RemoveTimeout(ctx context.Context, rideID string) error {
	return r.client.ZRem(ctx, TimeoutZSetKey, rideID).Err()
}

func (r *redisDispatchRepo) WasProcessed(ctx context.Context, eventID string) (bool, error) {
	key := MatchingIdempotencyKeyPrefix + eventID
	n, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *redisDispatchRepo) MarkProcessed(ctx context.Context, eventID string) error {
	key := MatchingIdempotencyKeyPrefix + eventID
	return r.client.Set(ctx, key, "1", 24*time.Hour).Err()
}
