CREATE KEYSPACE IF NOT EXISTS echoride_location
WITH replication = {'class': 'SimpleStrategy', 'replication_factor': '1'};

CREATE TABLE IF NOT EXISTS echoride_location.t_driver_locations (
    ride_id uuid,
    created_at timestamp,
    driver_id uuid,
    lat double,
    lng double,
    distance_km double,
    PRIMARY KEY (ride_id, created_at)
) WITH CLUSTERING ORDER BY (created_at DESC);