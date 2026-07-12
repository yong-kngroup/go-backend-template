package media

import (
	"context"
	"fmt"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	model "github.com/freeDog-wy/go-backend-template/internal/model/media"
	"github.com/freeDog-wy/go-backend-template/internal/repository/media"
	"github.com/google/uuid"
	"path/filepath"
	"strings"
	"time"
)

const maxImageBytes int64 = 10 * 1024 * 1024

type Storage interface {
	ObjectKey(string) string
	PresignUpload(context.Context, string, string) (*PresignedUpload, error)
	HeadObject(context.Context, string) (*ObjectInfo, error)
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
type Service struct {
	tx      shared.TxManager
	repo    *media.Repository
	storage Storage
}

func New(tx shared.TxManager, repo *media.Repository, storage Storage) *Service {
	return &Service{tx: tx, repo: repo, storage: storage}
}

type UploadRequest struct {
	Filename, ContentType string
	SizeBytes             int64
	UserID                uint
}
type UploadResult struct {
	ID        uint              `json:"id"`
	ObjectKey string            `json:"object_key"`
	UploadURL string            `json:"upload_url"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expires_at"`
	Status    string            `json:"status"`
}

func (s *Service) RequestUpload(ctx context.Context, r UploadRequest) (*UploadResult, error) {
	if s.storage == nil {
		return nil, fmt.Errorf("object storage is not configured")
	}
	if r.UserID == 0 || r.SizeBytes <= 0 || r.SizeBytes > maxImageBytes || !allowed(r.ContentType) {
		return nil, fmt.Errorf("invalid media upload request")
	}
	ext := filepath.Ext(r.Filename)
	key := s.storage.ObjectKey("media/" + time.Now().UTC().Format("2006/01") + "/" + uuid.NewString() + ext)
	a := &model.Asset{UploaderUserID: r.UserID, ObjectKey: key, OriginalFilename: filepath.Base(r.Filename), MimeType: r.ContentType, SizeBytes: r.SizeBytes, Status: "pending"}
	var p *PresignedUpload
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		var e error
		if e = s.repo.Create(ctx, a); e != nil {
			return e
		}
		p, e = s.storage.PresignUpload(ctx, key, r.ContentType)
		return e
	})
	if err != nil {
		return nil, err
	}
	return &UploadResult{ID: a.ID, ObjectKey: key, UploadURL: p.URL, Headers: p.Headers, ExpiresAt: p.ExpiresAt, Status: a.Status}, nil
}
func (s *Service) Complete(ctx context.Context, id, userID uint) error {
	a, err := s.repo.Find(ctx, id)
	if err != nil {
		return err
	}
	if a.UploaderUserID != userID || a.Status != "pending" {
		return fmt.Errorf("media cannot be completed")
	}
	info, err := s.storage.HeadObject(ctx, a.ObjectKey)
	if err != nil {
		return s.repo.MarkFailed(ctx, id)
	}
	if !allowed(info.ContentType) || info.Size <= 0 || info.Size > maxImageBytes {
		return s.repo.MarkFailed(ctx, id)
	}
	return s.repo.MarkReady(ctx, id, info.ContentType, info.Size)
}
func allowed(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "image/jpeg", "image/png", "image/webp", "image/avif":
		return true
	}
	return false
}
