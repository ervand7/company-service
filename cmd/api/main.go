package main

import (
	"context"
	"errors"
	nethttp "net/http"
	"os"
	"os/signal"
	"syscall"

	appcompany "company-service/internal/application/company"
	"company-service/internal/config"
	"company-service/internal/infrastructure/auth"
	"company-service/internal/infrastructure/events"
	"company-service/internal/infrastructure/postgres"
	httpapi "company-service/internal/interfaces/http"

	_ "company-service/docs"

	"github.com/rs/zerolog"
)

// @title Company Service API
// @version 1.0
// @description REST API for managing companies.
// @host localhost:8080
// @BasePath /
// @schemes http
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and a JWT token.
func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load config")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	dbPool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer dbPool.Close()

	eventProducer := events.NewLogProducer(logger)
	repository := postgres.NewCompanyRepository(dbPool)
	service := appcompany.NewService(repository, eventProducer, &logger)
	jwtManager := auth.NewJWTManager(cfg.Auth.JWTSecret)

	server := &nethttp.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      httpapi.NewRouter(service, jwtManager, dbPool, logger),
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	go func() {
		logger.Info().Str("addr", cfg.HTTP.Addr).Msg("starting HTTP server")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, nethttp.ErrServerClosed) {
			logger.Error().Err(err).Msg("HTTP server failed")
			cancel()
		}
	}()

	<-ctx.Done()
	logger.Info().Msg("shutdown signal received")

	shutdownCtx, shutdownCtxCancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer shutdownCtxCancel()

	logger.Info().Msg("stopping HTTP server")
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("HTTP shutdown failed")
	}

	logger.Info().Msg("closing event producer")
	if err := eventProducer.Close(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("event producer close failed")
	}

	logger.Info().Msg("closing database pool")
	dbPool.Close()
	logger.Info().Msg("shutdown complete")
}
