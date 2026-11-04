package cassandra

import (
	"echo-ride/services/location-service/config"
	"time"

	"github.com/gocql/gocql"
	"go.uber.org/zap"
)

func NewCassandraSession(cfg config.CassandraConfig, logger *zap.Logger) (*gocql.Session, error) {
	cluster := gocql.NewCluster(cfg.Host)
	cluster.Keyspace = cfg.Keyspace

	// production use gocql.LocalQuorum
	cluster.Consistency = gocql.One
	cluster.Timeout = 5 * time.Second
	cluster.ConnectTimeout = 10 * time.Second

	cluster.RetryPolicy = &gocql.ExponentialBackoffRetryPolicy{
		NumRetries: 3,
		Min:        100 * time.Millisecond,
		Max:        10 * time.Second,
	}

	session, err := cluster.CreateSession()
	if err != nil {
		logger.Error("Failed to connect to Cassandra cluster", zap.String("hosts", cfg.Host), zap.Error(err))
		return nil, err
	}

	logger.Info("Successfully connected to Cassandra", zap.String("hosts", cfg.Host), zap.String("keyspace", cfg.Keyspace))
	return session, nil
}
