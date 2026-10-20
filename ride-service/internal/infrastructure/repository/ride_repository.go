package repository

import (
	"context"
	"echo-ride/services/ride-service/internal/domain"
	"echo-ride/services/ride-service/internal/infrastructure/db/dbgen"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RideRepositoryImpl struct {
	db *pgxpool.Pool
	q  *dbgen.Queries
}

func NewRideRepository(db *pgxpool.Pool) domain.RideRepository {
	return &RideRepositoryImpl{
		db: db,
		q:  dbgen.New(db),
	}
}

func toPgtypeUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func toUUID(id pgtype.UUID) uuid.UUID {
	return uuid.UUID(id.Bytes)
}

func toPgtypeNumeric(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%f", f))
	return n
}

func numericToFloat64(n pgtype.Numeric) float64 {
	f, _ := n.Float64Value()
	return f.Float64
}

func dbRideToDomain(dbRide dbgen.TRide) *domain.Ride {
	var driverID *uuid.UUID
	if dbRide.RideDriverID.Valid {
		id := toUUID(dbRide.RideDriverID)
		driverID = &id
	}

	return &domain.Ride{
		ID:         toUUID(dbRide.RideID),
		RiderID:    toUUID(dbRide.RideRiderID),
		DriverID:   driverID,
		PickupLat:  numericToFloat64(dbRide.RidePickupLat),
		PickupLng:  numericToFloat64(dbRide.RidePickupLng),
		DropoffLat: numericToFloat64(dbRide.RideDropoffLat),
		DropoffLng: numericToFloat64(dbRide.RideDropoffLng),
		Status:     domain.RideStatus(dbRide.RideStatus),
		Price:      numericToFloat64(dbRide.RidePrice),
	}
}

func (r *RideRepositoryImpl) Create(ctx context.Context, ride *domain.Ride, eventType string, eventPayload []byte) (*domain.Ride, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer tx.Rollback(ctx)

	// query runner with transaction
	qtx := r.q.WithTx(tx)

	rideParams := dbgen.CreateRideParams{
		RideRiderID:    toPgtypeUUID(ride.RiderID),
		RidePickupLat:  toPgtypeNumeric(ride.PickupLat),
		RidePickupLng:  toPgtypeNumeric(ride.PickupLng),
		RideDropoffLat: toPgtypeNumeric(ride.DropoffLat),
		RideDropoffLng: toPgtypeNumeric(ride.DropoffLng),
		RidePrice:      toPgtypeNumeric(ride.Price),
	}

	dbRide, err := qtx.CreateRide(ctx, rideParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create ride: %w", err)
	}

	outboxParams := dbgen.CreateOutboxEventParams{
		EventAggregateID:   toUUID(dbRide.RideID).String(),
		EventAggregateType: "ride",
		EventType:          eventType,
		EventPayload:       eventPayload,
	}

	_, err = qtx.CreateOutboxEvent(ctx, outboxParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	ride.ID = toUUID(dbRide.RideID)
	ride.Status = domain.RideStatus(dbRide.RideStatus)

	return ride, nil
}

func (r *RideRepositoryImpl) GetByID(ctx context.Context, id uuid.UUID) (*domain.Ride, error) {
	dbRide, err := r.q.GetRideByID(ctx, toPgtypeUUID(id))
	if err != nil {
		return nil, err
	}

	return dbRideToDomain(dbRide), nil
}

func (r *RideRepositoryImpl) ListRides(ctx context.Context, filter domain.RideFilter) ([]*domain.Ride, error) {
	params := dbgen.ListRidesParams{
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}

	if filter.RiderID != nil {
		params.RiderID = toPgtypeUUID(*filter.RiderID)
	}

	if filter.DriverID != nil {
		params.DriverID = toPgtypeUUID(*filter.DriverID)
	}

	if filter.Status != nil {
		params.Status = dbgen.NullRideStatus{
			RideStatus: dbgen.RideStatus(*filter.Status),
			Valid:      true,
		}
	}

	rows, err := r.q.ListRides(ctx, params)
	if err != nil {
		return nil, err
	}

	result := make([]*domain.Ride, 0, len(rows))
	for _, row := range rows {
		result = append(result, dbRideToDomain(row))
	}

	return result, nil
}

func (r *RideRepositoryImpl) AcceptRide(ctx context.Context, rideID, driverID uuid.UUID, eventType string, eventPayload []byte) (*domain.Ride, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer tx.Rollback(ctx)

	qtx := r.q.WithTx(tx)

	acceptParams := dbgen.AcceptRideParams{
		RideID:       toPgtypeUUID(rideID),
		RideDriverID: toPgtypeUUID(driverID),
	}

	dbRide, err := qtx.AcceptRide(ctx, acceptParams)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("ride not found or already accepted: %w", err)
		}
		return nil, fmt.Errorf("failed to accept ride: %w", err)
	}

	outboxParams := dbgen.CreateOutboxEventParams{
		EventAggregateID:   toUUID(dbRide.RideID).String(),
		EventAggregateType: string(domain.OutboxStateRide),
		EventType:          eventType,
		EventPayload:       eventPayload,
	}

	_, err = qtx.CreateOutboxEvent(ctx, outboxParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return dbRideToDomain(dbRide), nil
}

func (r *RideRepositoryImpl) UpdateStatus(ctx context.Context, rideID uuid.UUID, status domain.RideStatus, eventType string, eventPayload []byte) (*domain.Ride, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer tx.Rollback(ctx)

	qtx := r.q.WithTx(tx)

	udpateParams := dbgen.UpdateRideStatusParams{
		RideID:     toPgtypeUUID(rideID),
		RideStatus: dbgen.RideStatus(status),
	}

	dbRide, err := qtx.UpdateRideStatus(ctx, udpateParams)
	if err != nil {
		return nil, fmt.Errorf("failed to update ride status: %w", err)
	}

	outboxParams := dbgen.CreateOutboxEventParams{
		EventAggregateID:   toUUID(dbRide.RideID).String(),
		EventAggregateType: "ride",
		EventType:          eventType,
		EventPayload:       eventPayload,
	}

	_, err = qtx.CreateOutboxEvent(ctx, outboxParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return dbRideToDomain(dbRide), nil
}

func (r *RideRepositoryImpl) UpdateTripStatus(ctx context.Context, rideID, driverID uuid.UUID, oldStatus, newStatus domain.RideStatus, eventType string, eventPayload []byte) (*domain.Ride, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := r.q.WithTx(tx)

	updateParams := dbgen.UpdateTripStatusParams{
		RideID:            toPgtypeUUID(rideID),
		DriverID:          toPgtypeUUID(driverID),
		ExpectedOldStatus: dbgen.RideStatus(oldStatus),
		NewStatus:         dbgen.RideStatus(newStatus),
	}

	dbRide, err := qtx.UpdateTripStatus(ctx, updateParams)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("conflict: cannot update trip status, invalid state or unauthorized: %w", err)
		}
		return nil, fmt.Errorf("failed to update trip status: %w", err)
	}

	outboxParams := dbgen.CreateOutboxEventParams{
		EventAggregateID:   toUUID(dbRide.RideID).String(),
		EventAggregateType: string(domain.OutboxStateRide),
		EventType:          eventType,
		EventPayload:       eventPayload,
	}

	_, err = qtx.CreateOutboxEvent(ctx, outboxParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return dbRideToDomain(dbRide), nil
}

func (r *RideRepositoryImpl) CancelRide(ctx context.Context, rideID uuid.UUID, eventType string, eventPayload []byte) (*domain.Ride, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := r.q.WithTx(tx)

	dbRide, err := qtx.CancelRide(ctx, toPgtypeUUID(rideID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("ride not found or not cancellable: %w", err)
		}
		return nil, fmt.Errorf("failed to cancel ride: %w", err)
	}

	outboxParams := dbgen.CreateOutboxEventParams{
		EventAggregateID:   toUUID(dbRide.RideID).String(),
		EventAggregateType: string(domain.OutboxStateRide),
		EventType:          eventType,
		EventPayload:       eventPayload,
	}

	if _, err := qtx.CreateOutboxEvent(ctx, outboxParams); err != nil {
		return nil, fmt.Errorf("failed to create outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return dbRideToDomain(dbRide), nil
}

func (r *RideRepositoryImpl) CreateOutboxEventOnly(ctx context.Context, aggregateID, aggregateType, eventType string, eventPayload []byte) error {
	outboxParams := dbgen.CreateOutboxEventParams{
		EventAggregateID:   aggregateID,
		EventAggregateType: aggregateType,
		EventType:          eventType,
		EventPayload:       eventPayload,
	}

	_, err := r.q.CreateOutboxEvent(ctx, outboxParams)
	if err != nil {
		return fmt.Errorf("failed to create outbox event: %w", err)
	}

	return nil
}
