// Package card implements card (and mobile money) payments via Pesapal v3.
//
// Pesapal is widely used across Kenya, Uganda, Tanzania, and Rwanda.
// It accepts Visa, Mastercard, M-Pesa, Airtel Money, and bank transfers
// through a single hosted checkout page.
//
// Flow:
//  1. Initiate()        — registers IPN URL (once, cached), submits order,
//                         returns a redirect URL to Pesapal's hosted page
//  2. Customer pays     — on Pesapal's secure hosted checkout page
//  3. Pesapal IPN       — POSTs notification to /api/v1/payments/card/callback
//  4. HandleCallback()  — calls GetTransactionStatus to verify, returns result
//  5. Pesapal redirect  — sends customer to PESAPAL_REDIRECT_URL on frontend
//
// Docs: https://developer.pesapal.com/how-to-integrate/e-commerce/api-30/api-reference
package card

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

const (
	// tokenCacheKey holds the short-lived Pesapal OAuth token.
	// Pesapal tokens expire in ~5 minutes; we cache for 4 to be safe.
	tokenCacheKey = "pesapal:access_token"
	tokenTTL      = 4 * time.Minute

	// ipnIDCacheKey holds the registered IPN ID.
	// This only changes if you change the IPN URL — cache for 30 days.
	ipnIDCacheKey = "pesapal:ipn_id"
	ipnIDTTL      = 30 * 24 * time.Hour
)