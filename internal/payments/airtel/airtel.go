package airtel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Bessmack/hardware-store-api/internal/payments"
	"github.com/Bessmack/hardware-store-api/pkg/cache"
	"github.com/Bessmack/hardware-store-api/pkg/logger"
)

// Cache keys and TTLs for Airtel's OAuth token. The token is valid for 60 minutes, but we refresh it every 50 minutes to avoid edge cases where a token expires during a transaction.
const (
	tokenCacheKey = "airtel:access_token"
	tokenTTL      = 50 * time.Minute
)

// Provider implements payments.Provider for Airtel Money.
// Docs: https://developers.airtel.africa/documentation
type Provider struct {
	clientID     string
	clientSecret string
	baseURL      string
	httpClient   *http.Client
	cache        *cache.Cache
	storeCreds   payments.StoreCredentialsReader
}

type Config struct {
	ClientID     string
	ClientSecret string
	BaseURL      string
}

func New(cfg Config, c *cache.Cache, storeCreds payments.StoreCredentialsReader) *Provider {
	return &Provider{
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		baseURL:      cfg.BaseURL,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
		cache:        c,
		storeCreds:   storeCreds,
	}
}

func (p *Provider) Name() string { return "airtel" }
