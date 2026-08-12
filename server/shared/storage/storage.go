package storage

import (
	"context"
	"io"
)

// ObjectStorage is the interface the service layer depends on.
// Any S3-compatible backend (B2, R2, AWS S3) implements this by satisfying
// the method set — no backend-specific type ever appears in service code.
type ObjectStorage interface {
	PutObject(ctx context.Context, key string, data []byte, contentType string) error
	// PutObjectStream uploads from an io.Reader with a known size — avoids
	// buffering the entire file in memory when proxying large uploads.
	PutObjectStream(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	GetObject(ctx context.Context, key string) ([]byte, error)
	// GetObjectStream returns an io.ReadCloser for the object data. The caller must close it.
	GetObjectStream(ctx context.Context, key string) (io.ReadCloser, error)
	DeleteObject(ctx context.Context, key string) error
	GetPresignedURL(ctx context.Context, key string) (string, error)
	GetPresignedUploadURL(ctx context.Context, key, contentType string) (string, error)
	GetCloudFrontSignedURL(ctx context.Context, domain, key, keyID, privateKeyPEM string) (string, error)
	StatObject(ctx context.Context, key string) (int64, error)
}
