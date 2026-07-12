package imagevalidate

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"strings"

	_ "golang.org/x/image/webp"
)

var (
	ErrUnsupportedContentType = errors.New("unsupported image content type")
	ErrInvalidImage           = errors.New("invalid image")
	ErrImageTooLarge          = errors.New("image is too large")
	ErrImageDimensions        = errors.New("invalid image dimensions")
	ErrContentTypeMismatch    = errors.New("image content type mismatch")
)

type Constraints struct {
	MaxBytes            int64
	MaxWidth, MaxHeight int
	MaxPixels           int64
	AllowedContentTypes []string
}

type Metadata struct {
	ContentType string
	Width       int
	Height      int
}

func SupportsContentType(contentType string, constraints Constraints) bool {
	contentType = NormalizeContentType(contentType)
	for _, allowed := range constraints.AllowedContentTypes {
		if contentType == NormalizeContentType(allowed) {
			return true
		}
	}
	return false
}

func NormalizeContentType(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// Validate reads and fully decodes one image, returning its actual metadata.
func Validate(reader io.Reader, declaredContentType string, objectSize int64, constraints Constraints) (Metadata, error) {
	if !SupportsContentType(declaredContentType, constraints) {
		return Metadata{}, ErrUnsupportedContentType
	}
	if objectSize <= 0 || objectSize > constraints.MaxBytes {
		return Metadata{}, ErrImageTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(reader, constraints.MaxBytes+1))
	if err != nil || int64(len(data)) != objectSize || int64(len(data)) > constraints.MaxBytes {
		return Metadata{}, ErrInvalidImage
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Metadata{}, ErrInvalidImage
	}
	contentType := contentTypeForFormat(format)
	if contentType == "" || !SupportsContentType(contentType, constraints) {
		return Metadata{}, ErrUnsupportedContentType
	}
	if contentType != NormalizeContentType(declaredContentType) {
		return Metadata{}, ErrContentTypeMismatch
	}
	if !validDimensions(config.Width, config.Height, constraints) {
		return Metadata{}, ErrImageDimensions
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return Metadata{}, ErrInvalidImage
	}
	return Metadata{ContentType: contentType, Width: config.Width, Height: config.Height}, nil
}

func validDimensions(width, height int, constraints Constraints) bool {
	if width <= 0 || height <= 0 || width > constraints.MaxWidth || height > constraints.MaxHeight {
		return false
	}
	return int64(width) <= constraints.MaxPixels/int64(height)
}

func contentTypeForFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	default:
		return ""
	}
}
