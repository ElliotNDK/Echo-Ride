-- name: CreateRide :one
INSERT INTO
    t_rides (
    ride_rider_id,
    ride_pickup_lat,
    ride_pickup_lng,
    ride_dropoff_lat,
    ride_dropoff_lng,
    ride_price
)
VALUES
    ($1, $2, $3, $4, $5, $6)
    RETURNING
  *;

-- name: GetRideByID :one
SELECT
    *
FROM
    t_rides
WHERE
    ride_id = $1
    LIMIT
  1;

-- name: ListRides :many
-- sqlc.nargs allows to pass optional parameters to the query. If the parameter is not provided, it will be NULL, and the condition will be ignored.
SELECT * FROM t_rides
WHERE (ride_rider_id = sqlc.narg('rider_id') OR sqlc.narg('rider_id') IS NULL)
    AND (ride_driver_id = sqlc.narg('driver_id') OR sqlc.narg('driver_id') IS NULL)
    AND (ride_status = sqlc.narg('status') OR sqlc.narg('status') IS NULL)
ORDER BY ride_created_at DESC
LIMIT $1 OFFSET $2;

-- -- name: AcceptRide :one
-- UPDATE t_rides
-- SET ride_status = 'ACCEPTED', ride_driver_id = $2, ride_updated_at = NOW()
-- WHERE ride_id = $1 AND ride_status = 'REQUESTED'
-- RETURNING *;

-- name: UpdateRideStatus :one
UPDATE t_rides
SET ride_status = $2, ride_updated_at = NOW()
WHERE ride_id = $1
RETURNING *;

-- name: AcceptRide :one
UPDATE t_rides
SET ride_status = 'ACCEPTED'::ride_status,
    ride_driver_id = sqlc.arg(ride_driver_id),
    ride_updated_at = NOW()
WHERE ride_id = sqlc.arg(ride_id)
  AND ride_status = 'REQUESTED'::ride_status
RETURNING *;

-- name: UpdateTripStatus :one
UPDATE t_rides
SET
    ride_status = sqlc.arg('new_status')::ride_status,
    ride_updated_at = NOW()
WHERE
    ride_id = sqlc.arg('ride_id')
    AND ride_driver_id = sqlc.arg('driver_id')
    AND ride_status = sqlc.arg('expected_old_status')::ride_status
RETURNING *;

-- name: CancelRide :one
-- CAS-style cancel: succeeds only if the ride is in a cancellable state
-- (REQUESTED, ACCEPTED, or IN_PROGRESS). Concurrent cancels race and only
-- one wins; subsequent attempts get pgx.ErrNoRows.
UPDATE t_rides
SET
    ride_status = 'CANCELLED'::ride_status,
    ride_updated_at = NOW()
WHERE
    ride_id = sqlc.arg('ride_id')
    AND ride_status IN ('REQUESTED'::ride_status, 'ACCEPTED'::ride_status, 'IN_PROGRESS'::ride_status)
RETURNING *;