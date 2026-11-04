-- +goose Up
SELECT 'up SQL query';
ALTER TABLE t_rides RENAME COLUMN ride_pickup_lon TO ride_pickup_lng;
ALTER TABLE t_rides RENAME COLUMN ride_dropoff_lon TO ride_dropoff_lng;

ALTER TYPE ride_status ADD VALUE IF NOT EXISTS 'FAILED';

-- +goose Down
SELECT 'down SQL query';
ALTER TABLE t_rides RENAME COLUMN ride_pickup_lng TO ride_pickup_lon;
ALTER TABLE t_rides RENAME COLUMN ride_dropoff_lng TO ride_dropoff_lon;
