package media

import (
	"context"
	"io"
	"time"
)

// Storage is the outbound port for media object storage.
type Storage interface {
	ObjectKey(name string) string
	PresignUpload(ctx context.Context, key, contentType string) (*PresignedUpload, error)
	HeadObject(ctx context.Context, key string) (*ObjectInfo, error)
	OpenObject(ctx context.Context, key string) (io.ReadCloser, error)
	DeleteObject(ctx context.Context, key string) error
	PublicURL(key string) string
}

type PresignedUpload struct {
	URL       string
	Headers   map[string]string
	ExpiresAt time.Time
}

type ObjectInfo struct {
	ContentType string
	Size        int64
}

type PublicAsset struct {
	ID        uint
	ObjectKey string
	URL       string
	AltText   string
	Title     string
}
