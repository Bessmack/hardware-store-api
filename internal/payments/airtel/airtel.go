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
