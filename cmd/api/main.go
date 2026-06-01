package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/Bessmack/hardware-store-api/internal/auth"
	"github.com/Bessmack/hardware-store-api/internal/config"
	"github.com/Bessmack/hardware-store-api/internal/geo"
	"github.com/Bessmack/hardware-store-api/internal/middleware"
	"github.com/Bessmack/hardware-store-api/internal/notifications"
	notifEmail "github.com/Bessmack/hardware-store-api/internal/notifications/email"
	notifWhatsApp "github.com/Bessmack/hardware-store-api/internal/notifications/whatsapp"
	cloudstorage "github.com/Bessmack/hardware-store-api/internal/storage/cloudinary"
	"github.com/Bessmack/hardware-store-api/internal/stores"
	"github.com/Bessmack/hardware-store-api/internal/users"
	"github.com/Bessmack/hardware-store-api/pkg/cache"
	"github.com/Bessmack/hardware-store-api/pkg/database"
	"github.com/Bessmack/hardware-store-api/pkg/logger"
)

func main() {
	// 1. Config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// 2. Logger
	logger.Init(cfg.App.Env)
	l := logger.Get()
	l.Info().Str("app", cfg.App.Name).Str("env", cfg.App.Env).Msg("starting server")

	ctx := context.Background()

	// 3. Database
	db, err := database.Connect(ctx, cfg.Database.URL)
	if err != nil {
		l.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()
	l.Info().Msg("database connected")

	// 4. Cache (Redis)
	cacheClient, err := cache.Connect(ctx, cfg.Redis.URL)
	if err != nil {
		l.Fatal().Err(err).Msg("failed to connect to redis")
	}
	defer cacheClient.Close()
	l.Info().Msg("cache connected")

	// 5. Storage (Cloudinary)
	storageClient, err := cloudstorage.New(cloudstorage.Config{
		CloudName: cfg.Cloudinary.CloudName,
		APIKey:    cfg.Cloudinary.APIKey,
		APISecret: cfg.Cloudinary.APISecret,
	})
	if err != nil {
		l.Fatal().Err(err).Msg("failed to initialise cloudinary")
	}
	l.Info().Msg("storage connected")
	_ = storageClient // used by pod domain (wired when built)

	// 6. Apply configurable business rules
	// Override package-level defaults with values from .env so behaviour
	// can be tuned without recompiling.
	geo.LocationTTL = time.Duration(cfg.Rules.LocationCacheTTLHours) * time.Hour

	// 7. Repositories
	userRepo  := users.NewRepository(db)
	storeRepo := stores.NewRepository(db)

	// 8. Services
	userService  := users.NewService(userRepo)
	storeService := stores.NewService(storeRepo)

	reverseGeocoder := geo.NewReverseGeocoder(
		cfg.Geo.OpenCageAPIKey,
		cfg.Geo.NominatimBaseURL,
		cfg.Geo.NominatimUserAgent,
	)
	// storeService implements geo.StoreLister - no circular import
	locationService := geo.NewLocationService(cacheClient, reverseGeocoder, storeService)
	geocoder        := geo.NewGeocoder(cfg.Geo.NominatimBaseURL, cfg.Geo.NominatimUserAgent)
	autocompleter   := geo.NewAutocompleter(cfg.Geo.PhotonBaseURL)

	authService := auth.NewService(userService, cacheClient, auth.ServiceConfig{
		JWTSecret:           cfg.JWT.Secret,
		AccessExpiryMinutes: cfg.JWT.AccessExpiryMinutes,
		RefreshExpiryDays:   cfg.JWT.RefreshExpiryDays,
	})

	// Notifications registry — register every channel; fan-out happens in service
	notifRegistry := notifications.NewRegistry()
	notifRegistry.Register(notifWhatsApp.New(notifWhatsApp.Config{
		APIURL:     cfg.WhatsApp.APIURL,
		MediaURL:   cfg.WhatsApp.MediaURL,
		IDInstance: cfg.WhatsApp.IDInstance,
		APIToken:   cfg.WhatsApp.APIToken,
		Phone:      cfg.WhatsApp.Phone,
	}))
	notifRegistry.Register(notifEmail.New(notifEmail.Config{
		Host:     cfg.Email.Host,
		Port:     cfg.Email.Port,
		User:     cfg.Email.User,
		Password: cfg.Email.Password,
		FromName: cfg.Email.FromName,
	}))
	notifService := notifications.NewService(notifRegistry)
	_ = notifService // injected into order, pod domains when built

	// 9. Middleware
	authMw       := middleware.NewAuthMiddleware(cfg.JWT.Secret, userService)
	storeScopeMw := middleware.NewStoreScopeMiddleware(userService)

	// 10. Handlers
	authHandler  := auth.NewHandler(authService, locationService)
	userHandler  := users.NewHandler(userService)
	storeHandler := stores.NewHandler(storeService)
	geoHandler   := geo.NewHandler(locationService, autocompleter, geocoder)

	// Suppress unused variable warnings for handlers not yet wired into routes
	_ = authMw
	_ = storeScopeMw
	_ = authHandler
	_ = userHandler
	_ = storeHandler
	_ = geoHandler

	// 11. Router
	// TODO: wire up server/routes.go as domains are added
	// router := server.NewRouter(cfg, authMw, storeScopeMw, authHandler, ...)

	// 12. HTTP Server
	srv := &http.Server{
		Addr: fmt.Sprintf(":%s", cfg.App.Port),
		// Handler: router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 13. Graceful shutdown
	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		l.Info().Str("port", cfg.App.Port).Msg("server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			l.Fatal().Err(err).Msg("server error")
		}
	}()

	<-shutdownCtx.Done()
	l.Info().Msg("shutdown signal received")

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(timeoutCtx); err != nil {
		l.Fatal().Err(err).Msg("forced shutdown")
	}

	l.Info().Msg("server stopped cleanly")
}