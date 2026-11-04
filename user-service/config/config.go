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
	Server   ServerConfig `yaml:"server"`
	Database DBConfig     `yaml:"database"`
	Kafka    KafkaConfig  `yaml:"kafka"`
	Jaeger   JaegerConfig `yaml:"jaeger"`
	Otel     OtelConfig   `yaml:"otel"`
	GRPC     GRPCConfig   `yaml:"grpc"`
}

type ServerConfig struct {
	Port string `yaml:"port" env:"USER_PORT" env-default:"8115" validate:"required"`
	Mode string `yaml:"mode" env:"USER_MODE" env-default:"development" validate:"required"`
}

type GRPCConfig struct {
	Port string `yaml:"port" env:"USER_GRPC_PORT" env-default:"9115" validate:"required"`
}

type DBConfig struct {
	Host     string `yaml:"host" env:"USER_DB_HOST" validate:"required"`
	Port     int    `yaml:"port" env:"USER_DB_PORT" validate:"required"`
	User     string `yaml:"username" env:"USER_DB_USER" validate:"required"`
	Password string `yaml:"password" env:"USER_DB_PASSWORD" validate:"required"`
	DBName   string `yaml:"dbname" env:"USER_DB_NAME" validate:"required"`
	Type     string `yaml:"type" env:"USER_DB_TYPE" env-default:"postgres" validate:"required,oneof=postgres mysql"`
}

type KafkaConfig struct {
	Brokers       []string `yaml:"brokers" env:"USER_KAFKA_BROKERS" validate:"required"`
	Topic         string   `yaml:"topic" env:"USER_KAFKA_TOPIC" validate:"required"`
	IdentityTopic string   `yaml:"identity_topic" env:"USER_KAFKA_IDENTITY_TOPIC" env-default:"auth.events" validate:"required"`
}

type JaegerConfig struct {
	AgentHost string `yaml:"service_host" env:"USER_JAEGER_AGENT_HOST" env-default:"localhost" validate:"required"`
	AgentPort int    `yaml:"service_port" env:"USER_JAEGER_AGENT_PORT" env-default:"4317" validate:"required"`
}

type OtelConfig struct {
	Endpoint         string  `yaml:"endpoint" env:"USER_OTEL_ENDPOINT" env-default:""`
	Insecure         bool    `yaml:"insecure" env:"USER_OTEL_INSECURE" env-default:"true"`
	TraceSampleRatio float64 `env:"USER_OTEL_TRACE_SAMPLE_RATIO" env-default:"1" validate:"gt=0,lte=1"`
}

func Load() (*Config, error) {
	envFile, explicit := os.LookupEnv("USER_CONFIG_PATH")
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
