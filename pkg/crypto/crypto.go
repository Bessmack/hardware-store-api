// Package crypto provides AES-256-GCM encryption for sensitive fields
// stored in the database (M-Pesa credentials, payment keys, etc.).
//
// How it works:
//   - Each encryption produces a unique random nonce (12 bytes)
//   - The nonce is prepended to the ciphertext before base64 encoding
//   - GCM provides authenticated encryption — tampering is detected on decrypt
//   - The encryption key never touches the database — it lives in .env only
//
// If the database is breached, the ciphertext is useless without the key.
// If the key is breached but not the database, the data is also safe.
// Both are needed together.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// Cipher holds the AES-256-GCM block cipher initialised from the encryption key.
type Cipher struct {
	gcm cipher.AEAD
}

// NewCipher creates a Cipher from a 32-byte hex-encoded key.
// The key should come from config.App.EncryptionKey which reads ENCRYPTION_KEY from .env.
//
// Generate a key with:
//
//	openssl rand -hex 32
//
// This produces a 64-character hex string representing 32 bytes (AES-256).
func NewCipher(hexKey string) (*Cipher, error) {
	if hexKey == "" {
		return nil, errors.New("crypto: ENCRYPTION_KEY is not set — cannot encrypt sensitive fields")
	}

	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("crypto: invalid ENCRYPTION_KEY — must be a 64-character hex string: %w", err)
	}
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("crypto: ENCRYPTION_KEY must be 32 bytes (64 hex chars), got %d bytes", len(keyBytes))
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to create GCM: %w", err)
	}

	return &Cipher{gcm: gcm}, nil
}

// Encrypt encrypts a plaintext string and returns a base64-encoded ciphertext.
// Each call produces a different output even for the same input because a
// fresh random nonce is generated every time.
//
// Returns an empty string unchanged — empty strings represent NULL credentials
// and there is nothing to encrypt.
func (c *Cipher) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: failed to generate nonce: %w", err)
	}

	// Seal appends the ciphertext to nonce, producing: [nonce][ciphertext][tag]
	sealed := c.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt decrypts a base64-encoded ciphertext back to the original plaintext.
// Returns an error if the ciphertext has been tampered with (GCM authentication
// failure) or is otherwise invalid.
//
// Returns an empty string unchanged.
func (c *Cipher) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("crypto: invalid base64 ciphertext: %w", err)
	}

	nonceSize := c.gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("crypto: ciphertext is too short to contain a nonce")
	}

	nonce, sealed := data[:nonceSize], data[nonceSize:]
	plaintext, err := c.gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		// GCM authentication failed — data was tampered with or the wrong key was used
		return "", errors.New("crypto: decryption failed — wrong key or corrupted ciphertext")
	}

	return string(plaintext), nil
}

// MustEncrypt encrypts plaintext and panics on error.
// Only use this in tests or one-off scripts — use Encrypt in production code.
func (c *Cipher) MustEncrypt(plaintext string) string {
	result, err := c.Encrypt(plaintext)
	if err != nil {
		panic(fmt.Sprintf("crypto: MustEncrypt failed: %v", err))
	}
	return result
}