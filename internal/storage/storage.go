package storage

import (
	"context"
	"io"
)

// Storage defines the interface for all file storage operations.
// Currently implemented by Cloudinary — swap the implementation in main.go without touching any other part of the codebase.
type Storage interface {
	// Upload stores a file and returns its Cloudinary public ID.
	// Parameter order: ctx, file, filename, folder
	// The public ID is what gets saved to the database (not the full URL).
	// Call URL(publicID) to get the CDN-served URL.
	Upload(ctx context.Context, file io.Reader, filename, folder string) (string, error)

	// Move relocates a file from one public ID to another.
	// Used to promote a disputed POD photo from delivery-photos/ to dispute-evidence/ before the auto-delete rule would remove it.
	Move(ctx context.Context, fromPublicID, toPublicID string) error

	// Delete permanently removes a file by its public ID.
	Delete(ctx context.Context, publicID string) error

	// URL returns the CDN-served URL for a given public ID.
	URL(publicID string) string
}

// Folder constants — all storage paths in the application go through these.
// Never hardcode folder strings outside this file.
const (
	// FolderDeliveryPhotos holds POD photos awaiting the dispute window.
	// Configure a 30-day auto-delete rule on this folder in the Cloudinary dashboard:
	//   Settings → Upload → Upload presets → add an expiry rule for delivery-photos/
	FolderDeliveryPhotos = "delivery-photos"

	// FolderDisputeEvidence holds photos that have been flagged in a dispute.
	// No auto-delete rule — these are kept until manually reviewed and cleared.
	FolderDisputeEvidence = "dispute-evidence"
)