package main

import (
	"context"
	"echo-ride/pkg/common"
	InitLogger "echo-ride/pkg/logger"
	"echo-ride/pkg/tracing"
	"echo-ride/services/auth-service/config"
	"echo-ride/services/auth-service/internal/application"
	db "echo-ride/services/auth-service/internal/infrastructure"
	grpcclient "echo-ride/services/auth-service/internal/infrastructure/grpc-client"
	"echo-ride/services/auth-service/internal/infrastructure/kafka"
	redisInfra "echo-ride/services/auth-service/internal/infrastructure/redis"
	"echo-ride/services/auth-service/internal/infrastructure/repository"
	authMiddleware "echo-ride/services/auth-service/internal/presentation/middleware"
	"echo-ride/services/auth-service/pkg/jwt"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"go.uber.org/zap"
)

const (
	serviceName = "auth-service"
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
		log.Error("auth service terminated", zap.Error(err))
		return 1
	}
	log.Info("auth service stopped gracefully")

	return 0
}

func run(ctx context.Context, cfg *config.Config, log *zap.Logger) error {
	log.Info("starting auth service", zap.String("mode", cfg.Server.Mode))

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

	dbCtx, cancelDB := context.WithTimeout(ctx, 5*time.Second)
	defer cancelDB()
	dbPool, err := db.NewPostgresPool(dbCtx, cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer dbPool.Close()
	log.Info("database connection successful")

	redisClient := redisInfra.NewRedisClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	defer redisClient.Close()
	pingCtx, cancelPing := context.WithTimeout(ctx, 3*time.Second)
	defer cancelPing()
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}
	blacklist := redisInfra.NewBlacklist(redisClient)

	userClient, err := grpcclient.NewUserServiceClient(cfg.UserService.GRPCAddr)
	if err != nil {
		return fmt.Errorf("failed to create user-service gRPC client: %w", err)
	}
	defer userClient.Close()

	identityRepo := repository.NewIdentityRepository(dbPool)
	refreshRepo := repository.NewRefreshTokenRepository(dbPool)
	tokenMaker := jwt.NewTokenMaker(cfg.JWT.SecretKey)

	accessTTL := time.Duration(cfg.JWT.AccessTokenTTLMin) * time.Minute
	refreshTTL := time.Duration(cfg.JWT.RefreshTokenTTLHours) * time.Hour

	registerUC := application.NewRegisterUseCase(identityRepo, log)
	loginUC := application.NewLoginUseCase(application.LoginUCDeps{
		IdentityRepo:    identityRepo,
		RefreshRepo:     refreshRepo,
		UserClient:      userClient,
		TokenMaker:      tokenMaker,
		AccessTokenTTL:  accessTTL,
		RefreshTokenTTL: refreshTTL,
		Logger:          log,
	})
	refreshUC := application.NewRefreshUseCase(application.RefreshUCDeps{
		RefreshRepo:     refreshRepo,
		UserClient:      userClient,
		TokenMaker:      tokenMaker,
		AccessTokenTTL:  accessTTL,
		RefreshTokenTTL: refreshTTL,
		Logger:          log,
	})
	logoutUC := application.NewLogoutUseCase(refreshRepo, blacklist, log)
	handleUserUC := application.NewHandleUserEventUseCase(refreshRepo, log)

	jwtAuth := authMiddleware.JWTAuth(tokenMaker, blacklist)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	authConsumer := kafka.NewAuthConsumer(cfg.Kafka, cfg.Kafka.UserTopic, handleUserUC, log)

	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		authConsumer.Start(workerCtx)
	}()
	defer func() {
		workerCancel()
		select {
		case <-consumerDone:
		case <-time.After(5 * time.Second):
			log.Warn("kafka consumer did not stop in time")
		}
		authConsumer.Close()
	}()

	srv := newServer(ServerConfig{
		DBPool:     dbPool,
		Config:     cfg,
		Logger:     log,
		RegisterUC: registerUC,
		LoginUC:    loginUC,
		RefreshUC:  refreshUC,
		LogoutUC:   logoutUC,
		JWTAuth:    jwtAuth,
	})

	srvAddr := fmt.Sprintf(":%s", cfg.Server.Port)
	httpSrv := http.Server{
		Addr:              srvAddr,
		Handler:           srv,
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

	var runErr error
	select {
	case <-ctx.Done():
		log.Info("shut down Auth Service...")
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
