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
	"github.com/Bessmack/hardware-store-api/internal/cart"
	"github.com/Bessmack/hardware-store-api/internal/config"
	"github.com/Bessmack/hardware-store-api/internal/delivery"
	"github.com/Bessmack/hardware-store-api/internal/geo"
	"github.com/Bessmack/hardware-store-api/internal/inventory"
	"github.com/Bessmack/hardware-store-api/internal/middleware"
	"github.com/Bessmack/hardware-store-api/internal/notifications"
	notifEmail "github.com/Bessmack/hardware-store-api/internal/notifications/email"
	notifWhatsApp "github.com/Bessmack/hardware-store-api/internal/notifications/whatsapp"
	"github.com/Bessmack/hardware-store-api/internal/orders"
	"github.com/Bessmack/hardware-store-api/internal/payments"
	"github.com/Bessmack/hardware-store-api/internal/payments/airtel"
	cardprovider "github.com/Bessmack/hardware-store-api/internal/payments/card"
	"github.com/Bessmack/hardware-store-api/internal/payments/mpesa"
	"github.com/Bessmack/hardware-store-api/internal/pod"
	"github.com/Bessmack/hardware-store-api/internal/reports"
	"github.com/Bessmack/hardware-store-api/internal/server"
	"github.com/Bessmack/hardware-store-api/internal/products"
	cloudstorage "github.com/Bessmack/hardware-store-api/internal/storage/cloudinary"
	"github.com/Bessmack/hardware-store-api/internal/stores"
	"github.com/Bessmack/hardware-store-api/internal/users"
	"github.com/Bessmack/hardware-store-api/internal/wishlist"
	"github.com/Bessmack/hardware-store-api/pkg/cache"
	"github.com/Bessmack/hardware-store-api/pkg/crypto"
	"github.com/Bessmack/hardware-store-api/pkg/database"
	"github.com/Bessmack/hardware-store-api/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type paymentInitiatorAdapter struct {
	service *payments.Service
}

func (a *paymentInitiatorAdapter) Initiate(ctx context.Context, req orders.PaymentInitRequest) (*orders.PaymentInitResult, error) {
	result, err := a.service.Initiate(ctx, payments.InitiateRequest{
		OrderID:        req.OrderID,
		StoreID:        req.StoreID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Phone:          req.Phone,
		Provider:       req.Provider,
		Description:    req.Description,
		PaymentChannel: payments.PaymentChannel(req.PaymentChannel),
	})
	if err != nil {
		return nil, err
	}
	return &orders.PaymentInitResult{
		ProviderRef:     result.ProviderRef,
		Instructions:    result.Instructions,
		AwaitingPayment: result.AwaitingPayment,
		RedirectURL:     result.RedirectURL,
	}, nil
}

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

	ctx := context.Background()

	// ── 3. Database ───────────────────────────────────────────────────────────
	db, err := database.Connect(ctx, cfg.Database.URL)
	if err != nil {
		l.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()
	l.Info().Msg("database connected")

	// Run migrations before starting the server to ensure the schema is up-to-date.
	// In production, consider running migrations as a separate step during deployment to avoid downtime on large tables.
	if err := database.RunMigrations(cfg.Database.URL, "./migrations"); err != nil {
		l.Fatal().Err(err).Msg("migrations failed")
	}
	l.Info().Msg("migrations applied")

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

	// ── 6. Apply configurable business rules ──────────────────────────────────

	// Override package-level defaults with values from .env so behaviour can be tuned without recompiling.
	geo.LocationTTL = time.Duration(cfg.Rules.LocationCacheTTLHours) * time.Hour

	// ── 7. Repositories ───────────────────────────────────────────────────────
	// All repositories must be declared before any service that depends on them.
	userRepo := users.NewRepository(db)
	cipher, err := crypto.NewCipher(cfg.Security.EncryptionKey)
	if err != nil {
		l.Fatal().Err(err).Msg("failed to initialize crypto cipher")
	}
	storeRepo     := stores.NewRepository(db, cipher)
	productRepo   := products.NewRepository(db)
	inventoryRepo := inventory.NewRepository(db)
	cartRepo      := cart.NewRepository(db)
	wishlistRepo  := wishlist.NewRepository(db)
	deliveryRepo  := delivery.NewRepository(db)
	orderRepo     := orders.NewRepository(db)
	podRepo       := pod.NewRepository(db)
	reportsRepo    := reports.NewRepository(db)

	// ── 8. Payments ───────────────────────────────────────────────────────────
	paymentRegistry := payments.NewRegistry()
	paymentRegistry.Register(airtel.New(airtel.Config{
		ClientID:     cfg.Airtel.ClientID,
		ClientSecret: cfg.Airtel.ClientSecret,
		BaseURL:      cfg.Airtel.BaseURL,
	}, cacheClient, storeRepo))
	paymentRegistry.Register(mpesa.New(mpesa.Config{
		ConsumerKey:      cfg.Mpesa.ConsumerKey,
		ConsumerSecret:   cfg.Mpesa.ConsumerSecret,
		BaseURL:          cfg.Mpesa.BaseURL,
		DefaultShortcode: cfg.Mpesa.Shortcode,
		DefaultPasskey:   cfg.Mpesa.Passkey,
	}, cacheClient, storeRepo))
	paymentRegistry.Register(cardprovider.New(cardprovider.Config{
		ConsumerKey:    cfg.Card.ConsumerKey,
		ConsumerSecret: cfg.Card.ConsumerSecret,
		BaseURL:        cfg.Card.BaseURL,
		CallbackURL:    cfg.Card.CallbackURL,
		RedirectURL:    cfg.Card.RedirectURL,
	}, cacheClient))

	// Airtel callback URL lives on our server — derive from M-Pesa callback URL
	airtelCallbackURL := ""
	if cfg.Mpesa.CallbackURL != "" {
		airtelCallbackURL = cfg.Mpesa.CallbackURL[:len(cfg.Mpesa.CallbackURL)-len("mpesa/callback")] + "airtel/callback"
	}
	paymentService := payments.NewService(paymentRegistry, storeRepo, payments.ServiceConfig{
		MpesaCallbackURL:  cfg.Mpesa.CallbackURL,
		AirtelCallbackURL: airtelCallbackURL,
		CardCallbackURL:   cfg.Card.CallbackURL,
	})

	// ── 9. Notifications ──────────────────────────────────────────────────────
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

	// ── 10. Services ──────────────────────────────────────────────────────────
	userService  := users.NewService(userRepo)
	storeService := stores.NewService(storeRepo)

	reverseGeocoder := geo.NewReverseGeocoder(
		cfg.Geo.OpenCageAPIKey,
		cfg.Geo.NominatimBaseURL,
		cfg.Geo.NominatimUserAgent,
	)
	locationService := geo.NewLocationService(cacheClient, reverseGeocoder, storeService)
	geocoder        := geo.NewGeocoder(cfg.Geo.NominatimBaseURL, cfg.Geo.NominatimUserAgent)
	autocompleter   := geo.NewAutocompleter(cfg.Geo.PhotonBaseURL)

	productService   := products.NewService(productRepo)
	inventoryService := inventory.NewService(inventoryRepo)

	cartService := cart.NewService(
		cartRepo,
		inventoryRepo, // cart.InventoryReader  — GetCurrentPrice
		deliveryRepo,  // cart.WeightThresholdReader — GetWeightThresholds
	)
	wishlistService := wishlist.NewService(
		wishlistRepo,
		inventoryRepo,
	)
	deliveryService := delivery.NewService(deliveryRepo, storeRepo)

	// Orders — payment and POD wired after construction via setters
	orderService := orders.NewService(
		orderRepo,
		cartRepo,        // orders.CartReader        — GetItemsForOrder, ClearCart
		inventoryRepo,   // orders.StockManager      — ReduceStock, RestoreStock
		deliveryService, // orders.DeliveryFeeCalculator — CalculateFee
		nil,             // orders.PaymentInitiator  — wired when payments domain is built (set below)
		nil,             // orders.PODDispatcher     — wired when POD domain is built (set below)
		storeRepo,       // orders.StoreInfoReader   — GetStoreInfo
		userRepo,        // orders.CustomerInfoReader — GetCustomerInfo
		notifService,    // orders.OrderNotifier
	)
	orderService.SetPaymentInitiator(&paymentInitiatorAdapter{service: paymentService})

	podService := pod.NewService(
		podRepo,
		orderRepo,     // pod.OrderReader
		orderService,  // pod.OrderStatusUpdater
		userRepo,      // pod.CustomerInfoReader
		notifService,  // pod.PODNotifier
		storageClient, // storage.Storage
		pod.ServiceConfig{
			OTPLength:          cfg.Rules.OTPLength,
			GPSToleranceMetres: float64(cfg.Rules.PODGPSToleranceMetres),
			DisputeWindowHours: cfg.Rules.DisputeWindowHours,
		},
	)
	orderService.SetPODDispatcher(podService)

	reportsService := reports.NewService(reportsRepo)

	authService := auth.NewService(userService, cacheClient, auth.ServiceConfig{
		JWTSecret:           cfg.JWT.Secret,
		AccessExpiryMinutes: cfg.JWT.AccessExpiryMinutes,
		RefreshExpiryDays:   cfg.JWT.RefreshExpiryDays,
	})

	// ── 11. Middleware ────────────────────────────────────────────────────────
	authMw       := middleware.NewAuthMiddleware(cfg.JWT.Secret, userService)
	storeScopeMw := middleware.NewStoreScopeMiddleware(userService)

	redisOpts, _ := redis.ParseURL(cfg.Redis.URL)
	redisRaw     := redis.NewClient(redisOpts)
	defer redisRaw.Close()

	rateLimiter := middleware.NewRateLimiter(redisRaw)
	corsMw      := middleware.CORS(middleware.CORSConfig{
		AppURL:        cfg.App.URL,
		IsDevelopment: cfg.IsDevelopment(),
	})

	// ── 12. Handlers ──────────────────────────────────────────────────────────
	authHandler      := auth.NewHandler(authService, locationService)
	userHandler      := users.NewHandler(userService)
	storeHandler     := stores.NewHandler(storeService)
	geoHandler       := geo.NewHandler(locationService, autocompleter, geocoder)
	productHandler   := products.NewHandler(productService, locationService)
	inventoryHandler := inventory.NewHandler(inventoryService)
	cartHandler      := cart.NewHandler(cartService)
	wishlistHandler  := wishlist.NewHandler(wishlistService, locationService)
	deliveryHandler  := delivery.NewHandler(deliveryService)
	orderHandler     := orders.NewHandler(orderService)
	podHandler       := pod.NewHandler(podService)
	paymentHandler   := payments.NewHandler(paymentRegistry, orderService)
	reportsHandler  := reports.NewHandler(reportsService)

	// ── 13. Router ────────────────────────────────────────────────────────────
	router := server.NewRouter(
		cfg,
		authMw, storeScopeMw, rateLimiter, corsMw,
		authHandler, userHandler, storeHandler, geoHandler,
		productHandler, inventoryHandler, cartHandler,
		wishlistHandler, deliveryHandler, orderHandler,
		podHandler, paymentHandler, reportsHandler,
	)

	// ── 14. HTTP Server ───────────────────────────────────────────────────────
	srv := &http.Server{
		Addr: fmt.Sprintf(":%s", cfg.App.Port),
		Handler: router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ── 15. Graceful shutdown ─────────────────────────────────────────────────
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
