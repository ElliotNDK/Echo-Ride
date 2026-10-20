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
	Server      ServerConfig      `yaml:"server"`
	Database    DBConfig          `yaml:"database"`
	Kafka       KafkaConfig       `yaml:"kafka"`
	Jaeger      JaegerConfig      `yaml:"jaeger"`
	Otel        OtelConfig        `yaml:"otel"`
	JWT         JWTConfig         `yaml:"jwt"`
	Redis       RedisConfig       `yaml:"redis"`
	UserService UserServiceConfig `yaml:"user_service"`
}

type ServerConfig struct {
	Port string `yaml:"port" env:"AUTH_PORT" env-default:"8114" validate:"required"`
	Mode string `yaml:"mode" env:"AUTH_MODE" env-default:"development" validate:"required"`
}

type DBConfig struct {
	Host     string `yaml:"host" env:"AUTH_DB_HOST" validate:"required"`
	Port     int    `yaml:"port" env:"AUTH_DB_PORT" validate:"required"`
	User     string `yaml:"username" env:"AUTH_DB_USER" validate:"required"`
	Password string `yaml:"password" env:"AUTH_DB_PASSWORD" validate:"required"`
	DBName   string `yaml:"dbname" env:"AUTH_DB_NAME" validate:"required"`
	Type     string `yaml:"type" env:"AUTH_DB_TYPE" env-default:"postgres" validate:"required,oneof=postgres mysql"`
}

type KafkaConfig struct {
	Brokers   []string `yaml:"brokers" env:"AUTH_KAFKA_BROKERS" validate:"required"`
	Topic     string   `yaml:"topic" env:"AUTH_KAFKA_TOPIC" validate:"required"`
	UserTopic string   `yaml:"user_topic" env:"AUTH_KAFKA_USER_TOPIC" env-default:"user.events" validate:"required"`
}

type JaegerConfig struct {
	AgentHost string `yaml:"service_host" env:"AUTH_JAEGER_AGENT_HOST" env-default:"localhost" validate:"required"`
	AgentPort int    `yaml:"service_port" env:"AUTH_JAEGER_AGENT_PORT" env-default:"4317" validate:"required"`
}

type OtelConfig struct {
	Endpoint         string  `yaml:"endpoint" env:"AUTH_OTEL_ENDPOINT" env-default:""`
	Insecure         bool    `yaml:"insecure" env:"AUTH_OTEL_INSECURE" env-default:"true"`
	TraceSampleRatio float64 `env:"AUTH_OTEL_TRACE_SAMPLE_RATIO" env-default:"1" validate:"gt=0,lte=1"`
}

type JWTConfig struct {
	SecretKey            string `yaml:"secret" env:"AUTH_JWT_SECRET" validate:"required"`
	AccessTokenTTLMin    int    `yaml:"access_token_ttl_min" env:"AUTH_JWT_ACCESS_TTL_MIN" env-default:"15" validate:"required"`
	RefreshTokenTTLHours int    `yaml:"refresh_token_ttl_hours" env:"AUTH_JWT_REFRESH_TTL_HOURS" env-default:"720" validate:"required"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr" env:"AUTH_REDIS_ADDR" env-default:"localhost:6379" validate:"required"`
	Password string `yaml:"password" env:"AUTH_REDIS_PASSWORD" env-default:""`
	DB       int    `yaml:"db" env:"AUTH_REDIS_DB" env-default:"0"`
}

type UserServiceConfig struct {
	GRPCAddr string `yaml:"grpc_addr" env:"AUTH_USER_SERVICE_GRPC_ADDR" env-default:"localhost:9115" validate:"required"`
}

func Load() (*Config, error) {
	envFile, explicit := os.LookupEnv("AUTH_CONFIG_PATH")
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
