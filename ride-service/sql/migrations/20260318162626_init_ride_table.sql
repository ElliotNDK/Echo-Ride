-- +goose Up
SELECT 'up SQL query';

CREATE TYPE ride_status AS ENUM (
    'REQUESTED',
    'ACCEPTED',
    'IN_PROGRESS',
    'COMPLETED',
    'CANCELLED'
);

CREATE TABLE t_rides (
    ride_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ride_rider_id UUID NOT NULL,
    ride_driver_id UUID,
    ride_pickup_lat DECIMAL(9,6) NOT NULL,
    ride_pickup_lon DECIMAL(9,6) NOT NULL,
    ride_dropoff_lat DECIMAL(9,6) NOT NULL,
    ride_dropoff_lon DECIMAL(9,6) NOT NULL,
    ride_status ride_status NOT NULL DEFAULT 'REQUESTED',
    ride_price DECIMAL(10,2) NOT NULL,
    ride_created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ride_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_t_rides_ride_rider_id ON t_rides (ride_rider_id);
CREATE INDEX idx_t_rides_ride_driver_id ON t_rides (ride_driver_id);

-- +goose Down
SELECT 'down SQL query';
DROP TABLE IF EXISTS t_rides;
DROP TYPE IF EXISTS ride_status;
