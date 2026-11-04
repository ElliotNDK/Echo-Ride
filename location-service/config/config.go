package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Redis     RedisConfig     `yaml:"redis"`
	Kafka     KafkaConfig     `yaml:"kafka"`
	Grpc      GrpcConfig      `yaml:"grpc"`
	Jaeger    JaegerConfig    `yaml:"jaeger"`
	Otel      OtelConfig      `yaml:"otel"`
	JWT       JWTConfig       `yaml:"jwt"`
	Cassandra CassandraConfig `yaml:"cassandra"`
	OSRM      OSRMConfig      `yaml:"osrm"`
}

type ServerConfig struct {
	Port string `yaml:"port" env:"LOCATION_PORT" env-default:"8112" validate:"required"`
	Mode string `yaml:"mode" env:"LOCATION_MODE" env-default:"development" validate:"required"`
}

type RedisConfig struct {
	Host     string `yaml:"host" env:"LOCATION_REDIS_HOST" validate:"required"`
	Port     int    `yaml:"port" env:"LOCATION_REDIS_PORT" validate:"required"`
	Password string `yaml:"password" env:"LOCATION_REDIS_PASSWORD"`
	DB       int    `yaml:"db" env:"LOCATION_REDIS_DB" env-default:"0"`
}

type KafkaConfig struct {
	Brokers []string `yaml:"brokers" env:"LOCATION_KAFKA_BROKERS" validate:"required"`
	Topic   string   `yaml:"topic" env:"LOCATION_KAFKA_TOPIC" validate:"required"`
	GroupID string   `yaml:"group_id" env:"LOCATION_KAFKA_GROUP_ID" validate:"required"`
}

type GrpcConfig struct {
	Port string `yaml:"port" env:"LOCATION_GRPC_PORT" env-default:"9090" validate:"required"`
}

type JaegerConfig struct {
	AgentHost string `yaml:"service_host" env:"LOCATION_JAEGER_AGENT_HOST" env-default:"localhost" validate:"required"`
	AgentPort int    `yaml:"service_port" env:"LOCATION_JAEGER_AGENT_PORT" env-default:"4317" validate:"required"`
}

type OtelConfig struct {
	Endpoint         string  `yaml:"endpoint" env:"LOCATION_OTEL_ENDPOINT" env-default:""`
	Insecure         bool    `yaml:"insecure" env:"LOCATION_OTEL_INSECURE" env-default:"true"`
	TraceSampleRatio float64 `env:"LOCATION_OTEL_TRACE_SAMPLE_RATIO" env-default:"1" validate:"gt=0,lte=1"`
}

type JWTConfig struct {
	SecretKey      string `yaml:"secret" env:"LOCATION_JWT_SECRET" validate:"required"`
	ExpirationTime int    `yaml:"expiration_time" env:"LOCATION_JWT_EXPIRATION_TIME" env-default:"1440" validate:"required"` // in minutes
}

type CassandraConfig struct {
	Host string `yaml:"host" env:"LOCATION_CASSANDRA_HOST" validate:"required"`
	//Port     int    `yaml:"port" env:"LOCATION_CASSANDRA_PORT" validate:"required"`
	Keyspace string `yaml:"keyspace" env:"LOCATION_CASSANDRA_KEYSPACE" validate:"required"`
	//User     string `yaml:"username" env:"LOCATION_CASSANDRA_USER" validate:"required"`
	//Password string `yaml:"password" env:"LOCATION_CASSANDRA_PASSWORD" validate:"required"`
}

type OSRMConfig struct {
	BaseURL string `yaml:"base_url" env:"LOCATION_OSRM_BASE_URL" env-default:"http://localhost:5000" validate:"required"`
}

func Load() (*Config, error) {
	envFile, explicit := os.LookupEnv("LOCATION_CONFIG_PATH")
	if !explicit || envFile == "" {
		envFile, explicit = ".env", false
	}

	if err := godotenv.Load(envFile); err != nil {
		if explicit || !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("load env from %q: %w", envFile, err)
		}
	}

	cfg := &Config{}
	if err := cleanenv.ReadEnv(cfg); err != nil {
		return nil, fmt.Errorf("read config from env: %w", err)
	}
	if err := validator.New().Struct(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}
