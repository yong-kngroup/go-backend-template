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
type ReadyMedia struct{ ID uint }

func (s *Service) FindReady(ctx context.Context, id uint) (*ReadyMedia, error) {
	a, err := s.repo.Find(ctx, id)
	if err != nil {
		return nil, err
	}
	if a.Status != "ready" {
		return nil, fmt.Errorf("media is not ready")
	}
	return &ReadyMedia{ID: a.ID}, nil
}
func (s *Service) IsReady(ctx context.Context, id uint) (bool, error) {
	_, err := s.FindReady(ctx, id)
	return err == nil, err
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
type MediaResult struct {
	ID                                            uint `json:"id"`
	ObjectKey, OriginalFilename, MimeType, Status string
	SizeBytes                                     int64     `json:"size_bytes"`
	CreatedAt                                     time.Time `json:"created_at"`
}

func (s *Service) List(ctx context.Context, page, perPage int) ([]MediaResult, int64, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	items, total, err := s.repo.List(ctx, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, err
	}
	out := make([]MediaResult, 0, len(items))
	for _, a := range items {
		out = append(out, MediaResult{ID: a.ID, ObjectKey: a.ObjectKey, OriginalFilename: a.OriginalFilename, MimeType: a.MimeType, Status: a.Status, SizeBytes: a.SizeBytes, CreatedAt: a.CreatedAt})
	}
	return out, total, nil
}
func (s *Service) UpsertTranslation(ctx context.Context, id uint, locale, alt, title string) error {
	if _, err := s.repo.Find(ctx, id); err != nil {
		return err
	}
	return s.repo.UpsertTranslation(ctx, &model.Translation{MediaID: id, Locale: locale, AltText: alt, Title: title})
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
	if s.storage == nil {
		return fmt.Errorf("object storage is not configured")
	}
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
