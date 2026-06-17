package server

import (
	"net/http"

	"github.com/Bessmack/hardware-store-api/internal/auth"
	"github.com/Bessmack/hardware-store-api/internal/cart"
	"github.com/Bessmack/hardware-store-api/internal/config"
	"github.com/Bessmack/hardware-store-api/internal/delivery"
	"github.com/Bessmack/hardware-store-api/internal/geo"
	"github.com/Bessmack/hardware-store-api/internal/inventory"
	"github.com/Bessmack/hardware-store-api/internal/middleware"
	"github.com/Bessmack/hardware-store-api/internal/orders"
	"github.com/Bessmack/hardware-store-api/internal/payments"
	"github.com/Bessmack/hardware-store-api/internal/pod"
	"github.com/Bessmack/hardware-store-api/internal/products"
	"github.com/Bessmack/hardware-store-api/internal/reports"
	"github.com/Bessmack/hardware-store-api/internal/stores"
	"github.com/Bessmack/hardware-store-api/internal/users"
	"github.com/Bessmack/hardware-store-api/internal/wishlist"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)
