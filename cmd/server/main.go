package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"

	"github.com/radityajayantara/go-observability-boilerplate/internal/config"
	"github.com/radityajayantara/go-observability-boilerplate/internal/handler"
	"github.com/radityajayantara/go-observability-boilerplate/internal/middleware"
	"github.com/radityajayantara/go-observability-boilerplate/internal/observability"
	"github.com/radityajayantara/go-observability-boilerplate/internal/repository"
	"github.com/radityajayantara/go-observability-boilerplate/internal/service"
)

func main() {
	cfg := config.Load()

	// Logger
	logger := observability.NewLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	// Tracer
	ctx := context.Background()
	shutdownTracer, err := observability.InitTracer(ctx, "auth-service", cfg.OTLPEndpoint)
	if err != nil {
		logger.Warn("failed to init tracer, continuing without tracing", "error", err)
	} else {
		defer shutdownTracer(ctx) //nolint:errcheck
	}

	// Metrics
	reg := prometheus.DefaultRegisterer
	metrics := observability.NewMetrics(reg)

	// PostgreSQL
	pool, err := pgxpool.New(ctx, cfg.DBURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error("failed to ping database", "error", err)
		os.Exit(1)
	}
	logger.Info("connected to PostgreSQL")

	// Run migration
	if err := runMigration(ctx, pool); err != nil {
		logger.Error("failed to run migration", "error", err)
		os.Exit(1)
	}

	// Redis
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.Error("failed to parse redis URL", "error", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(redisOpts)
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Error("failed to ping redis", "error", err)
		os.Exit(1)
	}
	logger.Info("connected to Redis")

	// Dependencies
	userRepo := repository.NewUserRepository(pool)
	tokenSvc := service.NewTokenService(cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL, redisClient)
	authSvc := service.NewAuthService(userRepo, tokenSvc)
	authHandler := handler.NewAuthHandler(authSvc)
	healthHandler := handler.NewHealthHandler(pool, redisClient)

	// Rate limiter
	rateLimiter := middleware.NewRateLimiter(redisClient, cfg.RateLimitMax, cfg.RateLimitWindow)

	// Router
	r := chi.NewRouter()

	// Global middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.Recovery(logger))
	r.Use(middleware.Logging(logger))
	r.Use(middleware.Tracing)
	r.Use(middleware.MetricsMiddleware(metrics))
	r.Use(rateLimiter.Middleware)

	// Ops endpoints
	r.Get("/health", healthHandler.Health)
	r.Get("/ready", healthHandler.Ready)
	r.Handle("/metrics", observability.Handler())

	// Auth endpoints
	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", authHandler.Register)
		r.Post("/login", authHandler.Login)
		r.Post("/refresh", authHandler.Refresh)

		// Protected endpoints
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(tokenSvc))
			r.Post("/logout", authHandler.Logout)
			r.Get("/me", authHandler.Me)
		})
	})

	// Server with graceful shutdown
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server
	go func() {
		logger.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}
	logger.Info("server stopped")
}

func runMigration(ctx context.Context, pool *pgxpool.Pool) error {
	migration, err := os.ReadFile("migrations/001_create_users.sql")
	if err != nil {
		return fmt.Errorf("reading migration file: %w", err)
	}
	_, err = pool.Exec(ctx, string(migration))
	if err != nil {
		return fmt.Errorf("executing migration: %w", err)
	}
	return nil
}
