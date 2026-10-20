package cassandra

import (
	"context"
	"echo-ride/services/location-service/internal/domain"

	"github.com/gocql/gocql"
)

type locationRepo struct {
	session *gocql.Session
}

func NewCassandraLocationRepository(session *gocql.Session) domain.LocationHistoryRepository {
	return &locationRepo{session: session}
}

func (l *locationRepo) SaveLocationBatch(ctx context.Context, locations []domain.DriverLocation) error {
	if len(locations) == 0 {
		return nil
	}

	batch := l.session.NewBatch(gocql.UnloggedBatch).WithContext(ctx)

	query := `INSERT INTO driver_locations (ride_id, created_at, driver_id, lat, lng, distance_km) 
	          VALUES (?, ?, ?, ?, ?, ?)`

	for _, loc := range locations {
		batch.Query(query, loc.RideID, loc.CreatedAt, loc.DriverID, loc.Lat, loc.Lng, loc.DistanceKm)
	}

	return l.session.ExecuteBatch(batch)
}
