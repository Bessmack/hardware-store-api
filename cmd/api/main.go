package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/Bessmack/hardware-store-api/internal/config"
	"github.com/Bessmack/hardware-store-api/internal/geo"
	cloudstorage "github.com/Bessmack/hardware-store-api/internal/storage/cloudinary"
	"github.com/Bessmack/hardware-store-api/pkg/cache"
	"github.com/Bessmack/hardware-store-api/pkg/database"
	"github.com/Bessmack/hardware-store-api/pkg/logger"
)

func main() {
	// ── 1. Config ─────────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// ── 2. Logger ─────────────────────────────────────────────────────────────
	logger.Init(cfg.App.Env)
	l := logger.Get()
	l.Info().Str("app", cfg.App.Name).Str("env", cfg.App.Env).Msg("starting server")

	// ── 3. Database ───────────────────────────────────────────────────────────
	ctx := context.Background()

	db, err := database.Connect(ctx, cfg.Database.URL)
	if err != nil {
		l.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()
	l.Info().Msg("database connected")

	// ── 4. Cache (Redis) ──────────────────────────────────────────────────────
	cacheClient, err := cache.Connect(ctx, cfg.Redis.URL)
	if err != nil {
		l.Fatal().Err(err).Msg("failed to connect to redis")
	}
	defer cacheClient.Close()
	l.Info().Msg("cache connected")

	// ── 5. Storage (Cloudinary) ───────────────────────────────────────────────
	storageClient, err := cloudstorage.New(cloudstorage.Config{
		CloudName: cfg.Cloudinary.CloudName,
		APIKey:    cfg.Cloudinary.APIKey,
		APISecret: cfg.Cloudinary.APISecret,
	})
	if err != nil {
		l.Fatal().Err(err).Msg("failed to initialise cloudinary")
	}
	l.Info().Msg("storage connected")

	// Suppress unused variable warnings until domains are wired in
	_ = db
	_ = cacheClient
	_ = storageClient

	// ── 6. Apply configurable business rules ──────────────────────────────────
	// Override package-level defaults with values from .env so behaviour
	// can be tuned without recompiling.
	geo.LocationTTL = time.Duration(cfg.Rules.LocationCacheTTLHours) * time.Hour

	// ── 6. Repositories ───────────────────────────────────────────────────────
	// TODO: initialise repositories here as domains are built
	// e.g. userRepo := users.NewRepository(db)

	// ── 7. Services ───────────────────────────────────────────────────────────
	// TODO: initialise services here
	// e.g. userService := users.NewService(userRepo)

	// ── 8. Handlers ───────────────────────────────────────────────────────────
	// TODO: initialise handlers here
	// e.g. userHandler := users.NewHandler(userService)

	// ── 9. Router ─────────────────────────────────────────────────────────────
	// TODO: wire up router
	// router := server.NewRouter(cfg, userHandler, ...)

	// ── 10. HTTP Server ───────────────────────────────────────────────────────
	srv := &http.Server{
		Addr: fmt.Sprintf(":%s", cfg.App.Port),
		// Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ── 11. Graceful shutdown ─────────────────────────────────────────────────
	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		l.Info().Str("port", cfg.App.Port).Msg("server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			l.Fatal().Err(err).Msg("server error")
		}
	}()

	// Block until SIGINT or SIGTERM
	<-shutdownCtx.Done()
	l.Info().Msg("shutdown signal received")

	// Give in-flight requests 10 seconds to finish
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(timeoutCtx); err != nil {
		l.Fatal().Err(err).Msg("forced shutdown")
	}

	l.Info().Msg("server stopped cleanly")
}