package main

import (
	"context"
	"echo-ride/pkg-library/common"
	pb "echo-ride/pkg-library/grpc/location/v1"
	InitLogger "echo-ride/pkg-library/logger"
	"echo-ride/pkg-library/tracing"
	"echo-ride/services/location-service/config"
	"echo-ride/services/location-service/internal/application"
	"echo-ride/services/location-service/internal/infrastructure/db/cassandra"
	"echo-ride/services/location-service/internal/infrastructure/kafka"
	"echo-ride/services/location-service/internal/infrastructure/osrm"
	redisInfra "echo-ride/services/location-service/internal/infrastructure/redis"
	cassandraInfra "echo-ride/services/location-service/internal/infrastructure/repository/cassandra"
	grpcLocation "echo-ride/services/location-service/internal/presentation/grpc"
	"echo-ride/services/location-service/internal/presentation/ws"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

const (
	serviceName = "location-service"
)

func main() {
	os.Exit(initMain())
}

func initMain() int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	context.AfterFunc(ctx, stop)

	logCfg := InitLogger.Config{
		ServiceName: serviceName,
		Version:     common.Version,
		Env:         cfg.Server.Mode,
		Level:       zap.InfoLevel,
	}
	if cfg.Otel.Endpoint != "" {
		logCfg.OTLP = &InitLogger.OTLPConfig{
			Endpoint: cfg.Otel.Endpoint,
			Insecure: cfg.Otel.Insecure,
		}
	}

	log, shutdownLog, err := InitLogger.New(ctx, logCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init logger: %v\n", err)
		return 1
	}

	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownLog(flushCtx)
		_ = log.Sync()
	}()

	if err := run(ctx, cfg, log); err != nil {
		log.Error("location service terminated", zap.Error(err))
		return 1
	}
	log.Info("location service stopped gracefully")

	return 0
}

func run(ctx context.Context, cfg *config.Config, log *zap.Logger) error {
	log.Info("starting location service", zap.String("mode", cfg.Server.Mode))

	tp, err := tracing.Init(ctx, tracing.Config{
		ServiceName: serviceName,
		Version:     common.Version,
		Env:         cfg.Server.Mode,
		Endpoint:    net.JoinHostPort(cfg.Jaeger.AgentHost, strconv.Itoa(cfg.Jaeger.AgentPort)),
		Insecure:    cfg.Otel.Insecure,
		SampleRatio: cfg.Otel.TraceSampleRatio,
	})
	if err != nil {
		return fmt.Errorf("init tracer: %w", err)
	}
	defer func() {
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(shCtx); err != nil {
			log.Error("tracer shutdown failed", zap.Error(err))
		}
	}()

	redisClient, _ := redisInfra.NewRedisClient(cfg.Redis)
	defer redisClient.Close()
	pingCtx, cancelPing := context.WithTimeout(ctx, 3*time.Second)
	defer cancelPing()
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	cassandraSession, err := cassandra.NewCassandraSession(cfg.Cassandra, log)
	if err != nil {
		return fmt.Errorf("failed to create Cassandra sessionL: %w", err)
	}
	defer cassandraSession.Close()

	redisLocationRepo := redisInfra.NewRedisLocationRepo(redisClient)
	cassandraLocationRepo := cassandraInfra.NewCassandraLocationRepository(cassandraSession)

	osrmClient := osrm.NewOSRMClient("http://localhost:5000")

	workerCtx, workerCancel := context.WithCancel(context.Background())

	batcher := application.NewLocationBatcher(cassandraLocationRepo, redisLocationRepo, log)
	go batcher.Start(workerCtx)

	locationCleaner := application.NewLocationCleaner(redisLocationRepo, 3*time.Minute, log)
	go locationCleaner.Start(workerCtx)

	hub := ws.NewHub(redisClient, log)
	go hub.Run()

	findDriverUC := application.NewFindDriversUseCase(redisLocationRepo, osrmClient)
	notifyDriverUC := application.NewNotifyDriverUseCase(hub, log)
	updateLocationUC := application.NewUpdateDriverLocationUseCase(batcher, hub, log)

	rideConsumer := kafka.NewRideConsumer(cfg.Kafka, notifyDriverUC, hub, log)
	//go rideConsumer.Start(workerCtx)

	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		rideConsumer.Start(workerCtx)
	}()
	defer func() {
		workerCancel()
		select {
		case <-consumerDone:
		case <-time.After(5 * time.Second):
			log.Warn("kafka consumer did not stop in time")
		}
		rideConsumer.Close()
	}()

	wsHandler := ws.NewHandler(hub, updateLocationUC, cfg.JWT.SecretKey, log)

	e := newServer(wsHandler, log)

	srvAddr := fmt.Sprintf(":%s", cfg.Server.Port)
	httpSrv := &http.Server{
		Addr:              srvAddr,
		Handler:           e,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("server is listening", zap.String("address", srvAddr))
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("failed to start server: %w", err)
		}
	}()

	grpcAddr := fmt.Sprintf(":%s", cfg.Grpc.Port)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatal("Failed to listen for gRPC", zap.Error(err))
	}

	grpcServer := grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))
	locationGrpcHandler := grpcLocation.NewLocationGrpcServer(findDriverUC, log)
	pb.RegisterLocationServiceServer(grpcServer, locationGrpcHandler)
	go func() {
		log.Info("gRPC server is listening", zap.String("address", grpcAddr))
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal("Failed to start gRPC server", zap.Error(err))
		}
	}()

	var runErr error
	select {
	case <-ctx.Done():
		log.Info("shut down Location Service...")
	case runErr = <-errCh:
	case <-consumerDone:
		runErr = errors.New("kafka consumer stopped unexpectedly")
	}

	ctxShutdown, cancelShutdown := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancelShutdown()
	if err := httpSrv.Shutdown(ctxShutdown); err != nil {
		_ = httpSrv.Close()
		runErr = errors.Join(runErr, fmt.Errorf("http shutdown: %w", err))
	}

	return runErr
}
