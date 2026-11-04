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
}

type ServerConfig struct {
	Port string `yaml:"port" env:"RIDE_PORT" env-default:"8111" validate:"required"`
	Mode string `yaml:"mode" env:"RIDE_MODE" env-default:"development" validate:"required"`
}

type DBConfig struct {
	Host     string `yaml:"host" env:"RIDE_DB_HOST" validate:"required"`
	Port     int    `yaml:"port" env:"RIDE_DB_PORT" validate:"required"`
	User     string `yaml:"username" env:"RIDE_DB_USER" validate:"required"`
	Password string `yaml:"password" env:"RIDE_DB_PASSWORD" validate:"required"`
	DBName   string `yaml:"dbname" env:"RIDE_DB_NAME" validate:"required"`
	Type     string `yaml:"type" env:"RIDE_DB_TYPE" env-default:"postgres" validate:"required,oneof=postgres mysql"`
}

type KafkaConfig struct {
	Brokers []string `yaml:"brokers" env:"RIDE_KAFKA_BROKERS" validate:"required"`
	Topic   string   `yaml:"topic" env:"RIDE_KAFKA_TOPIC" validate:"required"`
}

type JaegerConfig struct {
	AgentHost string `yaml:"service_host" env:"RIDE_JAEGER_AGENT_HOST" env-default:"localhost" validate:"required"`
	AgentPort int    `yaml:"service_port" env:"RIDE_JAEGER_AGENT_PORT" env-default:"4317" validate:"required"`
}

type OtelConfig struct {
	Endpoint         string  `yaml:"endpoint" env:"RIDE_OTEL_ENDPOINT" env-default:""`
	Insecure         bool    `yaml:"insecure" env:"RIDE_OTEL_INSECURE" env-default:"true"`
	TraceSampleRatio float64 `env:"RIDE_OTEL_TRACE_SAMPLE_RATIO" env-default:"1" validate:"gt=0,lte=1"`
}

func Load() (*Config, error) {
	envFile, explicit := os.LookupEnv("RIDE_CONFIG_PATH")
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
