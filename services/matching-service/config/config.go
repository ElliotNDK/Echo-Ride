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
	Server       ServerConfig       `yaml:"server"`
	Redis        RedisConfig        `yaml:"redis"`
	Kafka        KafkaConfig        `yaml:"kafka"`
	Dependencies DependenciesConfig `yaml:"dependencies"`
	Jaeger       JaegerConfig       `yaml:"jaeger"`
	Otel         OtelConfig         `yaml:"otel"`
}

type ServerConfig struct {
	Port string `yaml:"port" env:"MATCHING_PORT" env-default:"8113" validate:"required"`
	Mode string `yaml:"mode" env:"MATCHING_MODE" env-default:"development" validate:"required"`
}

type RedisConfig struct {
	Host     string `yaml:"host" env:"MATCHING_REDIS_HOST" validate:"required"`
	Port     int    `yaml:"port" env:"MATCHING_REDIS_PORT" validate:"required"`
	Password string `yaml:"password" env:"MATCHING_REDIS_PASSWORD"`
	DB       int    `yaml:"db" env:"MATCHING_REDIS_DB" env-default:"0"`
}

type KafkaConfig struct {
	Brokers []string `yaml:"brokers" env:"MATCHING_KAFKA_BROKERS" validate:"required"`
	Topic   string   `yaml:"topic" env:"MATCHING_KAFKA_TOPIC" validate:"required"`
	GroupID string   `yaml:"group_id" env:"MATCHING_KAFKA_GROUP_ID" validate:"required"`
}

type DependenciesConfig struct {
	LocationGrpcUrl string `yaml:"location_grpc_url" env:"LOCATION_GRPC_URL" validate:"required"`
}

type JaegerConfig struct {
	AgentHost string `yaml:"service_host" env:"MATCHING_JAEGER_AGENT_HOST" env-default:"localhost" validate:"required"`
	AgentPort int    `yaml:"service_port" env:"MATCHING_JAEGER_AGENT_PORT" env-default:"4317" validate:"required"`
}

type OtelConfig struct {
	Endpoint         string  `yaml:"endpoint" env:"MATCHING_OTEL_ENDPOINT" env-default:""`
	Insecure         bool    `yaml:"insecure" env:"MATCHING_OTEL_INSECURE" env-default:"true"`
	TraceSampleRatio float64 `env:"MATCHING_OTEL_TRACE_SAMPLE_RATIO" env-default:"1" validate:"gt=0,lte=1"`
}

func Load() (*Config, error) {
	envFile, explicit := os.LookupEnv("MATCHING_CONFIG_PATH")
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
