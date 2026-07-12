package imagevalidate

import (
	"bytes"
	"encoding/base64"
	"testing"
)

var testConstraints = Constraints{
	MaxBytes:            10 * 1024 * 1024,
	MaxWidth:            8192,
	MaxHeight:           8192,
	MaxPixels:           16 * 1024 * 1024,
	AllowedContentTypes: []string{"image/jpeg", "image/png", "image/webp"},
}

func TestValidateAcceptsWebP(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString("UklGRrIBAABXRUJQVlA4TKUBAAAvSsAYAA8w//M///MfeJAkbXvaSG7m8Q3GfYSBJekwQztm/IcZlgwnmWImn2BK7aFmBtnVir6q//8VOkFE/xm4baTIu8c48ArEo6+B3zFKYln3pqClSCKX0begFTAXFOLXHSyF8cCNcZEG4OywuA4KVVfJCiArU7GAgJI8+lJP/OKMT/fBAjevg1cYB7YVkFuWga2lyPi5I0HFy5YTpWIHg0RZpkniRVW9odHAKOwosWuOGdxIyn2OvaCDvhg/we6TwadPBPbqBV58MsLmMJ8yZnOWk8SRz4N+QoyPL+MnamzMvcE1rHNEr91F9GKZPVUcS9w7PhhH36suB9qPeYb/oLk6cuTiJ0wOK3m5h1cKjW6EVZCYMK7dxcKCBdgP9HkKr9gkAO2P8GKZGWVdIAatQa+1IDpt6qyorVwdy01xdW8Jkfk6xjEXmVQQ+HQdFr6OKhIN34dXWq0+0qr6EJSCeeVLH9+gvGTLyqM65PQ44ihzlTXxQKjKbAvshXgir7Lil9w4L2bvMycmjQcqXaMCO6BlY28i+FOLzbfI1vEqxAhotocAAA==")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := Validate(bytes.NewReader(data), "image/webp", int64(len(data)), testConstraints)
	if err != nil {
		t.Fatalf("validate WebP: %v", err)
	}
	if metadata.ContentType != "image/webp" || metadata.Width <= 0 || metadata.Height <= 0 {
		t.Fatalf("WebP metadata = %#v", metadata)
	}
}

func TestValidateRejectsMismatchedContent(t *testing.T) {
	_, err := Validate(bytes.NewReader([]byte("not an image")), "image/png", int64(len("not an image")), testConstraints)
	if err != ErrInvalidImage {
		t.Fatalf("validation error = %v, want %v", err, ErrInvalidImage)
	}
}

func TestSupportsContentTypeRejectsAVIF(t *testing.T) {
	if SupportsContentType("image/avif", testConstraints) {
		t.Fatal("AVIF is allowed")
	}
}
