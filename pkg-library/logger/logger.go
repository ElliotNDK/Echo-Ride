package logger

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	ProductionMode  = "production"
	DevelopmentMode = "development"
)

type Config struct {
	ServiceName string
	Version     string
	Env         string
	Level       zapcore.Level // "production", "staging", "development"
	SetGlobal   bool

	OTLP *OTLPConfig
}

type OTLPConfig struct {
	Endpoint      string        // e.g. "otel-collector:4317"
	Insecure      bool          // plaintext gRPC
	ExportTimeout time.Duration // per export attempt; default 5s
	QueueSize     int           // bounded buffer; default 2048
	ExportEvery   time.Duration // batch interval; default 1s
}

type ShutdownFunc func(ctx context.Context) error

func New(ctx context.Context, cfg Config) (*zap.Logger, ShutdownFunc, error) {
	if cfg.ServiceName == "" {
		return nil, nil, fmt.Errorf("logger: Service name is required")
	}

	var (
		encCfg zapcore.EncoderConfig
		enc    zapcore.Encoder
	)
	if cfg.Env == DevelopmentMode {
		encCfg = zap.NewDevelopmentEncoderConfig()
		encCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		enc = zapcore.NewConsoleEncoder(encCfg)
	} else {
		encCfg = zap.NewProductionEncoderConfig()
		encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		enc = zapcore.NewJSONEncoder(encCfg)
	}

	stdout := zapcore.NewCore(enc, zapcore.Lock(os.Stdout), cfg.Level)
	cores := []zapcore.Core{stdout}
	shutdown := ShutdownFunc(func(ctx context.Context) error {
		return nil
	})

	if cfg.OTLP != nil {
		core, sd, err := newOTLPCore(ctx, cfg)
		if err != nil {
			return nil, nil, fmt.Errorf("logger: failed to init OTLP core: %w", err)
		}
		cores = append(cores, core)
		shutdown = sd
	}

	l := zap.New(
		zapcore.NewTee(cores...),
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
		zap.Fields(
			zap.String("service", cfg.ServiceName),
			zap.String("version", cfg.Version),
			zap.String("env", cfg.Env),
		),
	)

	if cfg.SetGlobal {
		zap.ReplaceGlobals(l)
	}

	return l, shutdown, nil

}

func newOTLPCore(ctx context.Context, cfg Config) (zapcore.Core, ShutdownFunc, error) {
	o := cfg.OTLP
	withDefaults(o)

	opts := []otlploggrpc.Option{
		otlploggrpc.WithEndpoint(o.Endpoint),
		otlploggrpc.WithTimeout(o.ExportTimeout),
		otlploggrpc.WithRetry(otlploggrpc.RetryConfig{
			Enabled:         true,
			InitialInterval: 500 * time.Millisecond,
			MaxInterval:     5 * time.Second,
			MaxElapsedTime:  30 * time.Second, // bounded: give up, drop the batch
		}),
	}
	if o.Insecure {
		opts = append(opts, otlploggrpc.WithInsecure())
	}

	exp, err := otlploggrpc.New(ctx, opts...)
	if err != nil {
		return nil, nil, err
	}

	res := resource.NewSchemaless(
		attribute.String("service.name", cfg.ServiceName),
		attribute.String("service.version", cfg.Version),
		attribute.String("deployment.environment", cfg.Env),
	)

	provider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exp,
			sdklog.WithMaxQueueSize(o.QueueSize),
			sdklog.WithExportInterval(o.ExportEvery),
			sdklog.WithExportTimeout(o.ExportTimeout),
		)),
	)

	core := otelzap.NewCore(cfg.ServiceName, otelzap.WithLoggerProvider(provider))

	return core, provider.Shutdown, nil
}

func withDefaults(o *OTLPConfig) {
	if o.ExportTimeout <= 0 {
		o.ExportTimeout = 5 * time.Second
	}
	if o.QueueSize <= 0 {
		o.QueueSize = 2048
	}
	if o.ExportEvery <= 0 {
		o.ExportEvery = time.Second
	}
}
