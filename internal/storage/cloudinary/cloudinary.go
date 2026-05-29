package cloudinary

import (
	"context"
	"fmt"
	"io"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

// boolPtr returns a pointer to the provided bool.
func boolPtr(b bool) *bool { return &b }

// Storage implements the storage.Storage interface using Cloudinary.
type Storage struct {
	cld       *cloudinary.Cloudinary
	cloudName string
}

// Config holds the Cloudinary credentials passed in from config.CloudinaryConfig.
type Config struct {
	CloudName string
	APIKey    string
	APISecret string
}

// New creates a new Cloudinary storage client and verifies the credentials.
func New(cfg Config) (*Storage, error) {
	cld, err := cloudinary.NewFromParams(cfg.CloudName, cfg.APIKey, cfg.APISecret)
	if err != nil {
		return nil, fmt.Errorf("cloudinary: failed to initialise client: %w", err)
	}

	return &Storage{
		cld:       cld,
		cloudName: cfg.CloudName,
	}, nil
}

// Upload stores a file in the given folder and returns its Cloudinary public ID.
//
// Transformations applied on upload:
//   - Resized to max 800px wide (preserves aspect ratio)
//   - Compressed to JPEG at quality 70
//
// This mirrors the client-side compression done in the delivery PWA — the server-side
// transformation acts as a safety net for any file that bypasses the client.
func (s *Storage) Upload(ctx context.Context, file io.Reader, folder, filename string) (string, error) {
	publicID := fmt.Sprintf("%s/%s", folder, filename)

	resp, err := s.cld.Upload.Upload(ctx, file, uploader.UploadParams{
		PublicID: publicID,
		Folder:   folder,
		// Resize + compress on upload
		Transformation: "w_800,q_70,f_jpg",
		// Tag for easy bulk operations (e.g. manual cleanup scripts)
		Tags: []string{"pod-photo"},
		// Overwrite if somehow the same filename is used twice
		Overwrite: boolPtr(true),
	})
	if err != nil {
		return "", fmt.Errorf("cloudinary: upload failed: %w", err)
	}

	return resp.PublicID, nil
}

// Move relocates a file to a new public ID (effectively a rename/folder change).
// Used to move a disputed photo from delivery-photos/ → dispute-evidence/
// before the 30-day auto-delete rule removes it.
func (s *Storage) Move(ctx context.Context, fromPublicID, toPublicID string) error {
	_, err := s.cld.Upload.Rename(ctx, uploader.RenameParams{
		FromPublicID: fromPublicID,
		ToPublicID:   toPublicID,
		Overwrite:    boolPtr(true),
		// Invalidate the CDN cache for the old URL
		Invalidate: boolPtr(true),
	})
	if err != nil {
		return fmt.Errorf("cloudinary: move failed (%s → %s): %w", fromPublicID, toPublicID, err)
	}

	return nil
}

// Delete permanently removes a file by its public ID.
// Also invalidates the CDN cache so the old URL stops serving the image.
func (s *Storage) Delete(ctx context.Context, publicID string) error {
	_, err := s.cld.Upload.Destroy(ctx, uploader.DestroyParams{
		PublicID:   publicID,
		Invalidate: boolPtr(true),
	})
	if err != nil {
		return fmt.Errorf("cloudinary: delete failed (%s): %w", publicID, err)
	}

	return nil
}

// URL returns the CDN URL for a given public ID.
// The URL is constructed without an API call — Cloudinary URLs are deterministic.
func (s *Storage) URL(publicID string) string {
	return fmt.Sprintf("https://res.cloudinary.com/%s/image/upload/%s", s.cloudName, publicID)
}
