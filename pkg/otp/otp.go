package otp

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// Generate returns a cryptographically secure numeric OTP of the given length.
// Uses crypto/rand — never math/rand — so the values are unpredictable.
//
// Usage:
//
//	code := otp.Generate(6)  // e.g. "847291"
func Generate(length int) string {
	const digits = "0123456789"
	result := make([]byte, length)

	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			// crypto/rand failure is a system-level problem — panic is appropriate
			panic(fmt.Sprintf("otp: failed to generate random number: %v", err))
		}
		result[i] = digits[n.Int64()]
	}

	return string(result)
}