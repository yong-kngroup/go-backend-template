package media

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	domainMedia "github.com/freeDog-wy/go-backend-template/internal/domain/media"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	model "github.com/freeDog-wy/go-backend-template/internal/model/media"
	"github.com/freeDog-wy/go-backend-template/internal/repository/media"
	"github.com/freeDog-wy/go-backend-template/pkg/imagevalidate"
	"github.com/google/uuid"
)

const maxImageBytes int64 = 10 * 1024 * 1024

const uploadCleanupLease = 5 * time.Minute

var imageConstraints = imagevalidate.Constraints{
	MaxBytes:            maxImageBytes,
	MaxWidth:            8192,
	MaxHeight:           8192,
	MaxPixels:           16 * 1024 * 1024,
	AllowedContentTypes: []string{"image/jpeg", "image/png", "image/webp"},
}

type Service struct {
	tx      shared.TxManager
	repo    *media.Repository
	storage domainMedia.Storage
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

func (s *Service) ListPublic(ctx context.Context, locale string, ids []uint) ([]domainMedia.PublicAsset, error) {
	if len(ids) == 0 || s.storage == nil {
		return []domainMedia.PublicAsset{}, nil
	}
	assets, err := s.repo.ListReadyPublic(ctx, locale, ids)
	if err != nil {
		return nil, err
	}
	result := make([]domainMedia.PublicAsset, 0, len(assets))
	for _, asset := range assets {
		asset.URL = strings.TrimSpace(s.storage.PublicURL(asset.ObjectKey))
		if asset.URL != "" {
			result = append(result, asset)
		}
	}
	return result, nil
}

func New(tx shared.TxManager, repo *media.Repository, storage domainMedia.Storage) *Service {
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
	var p *domainMedia.PresignedUpload
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		var e error
		if e = s.repo.Create(ctx, a); e != nil {
			return e
		}
		p, e = s.storage.PresignUpload(ctx, key, r.ContentType)
		if e != nil {
			return e
		}
		a.UploadExpiresAt = &p.ExpiresAt
		return s.repo.SetUploadExpiresAt(ctx, a.ID, p.ExpiresAt)
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
	now := time.Now().UTC()
	if a.UploadExpiresAt != nil && !a.UploadExpiresAt.After(now) {
		if err := s.repo.MarkExpired(ctx, id, now); err != nil {
			return err
		}
		return ErrMediaUploadExpired
	}
	info, err := s.storage.HeadObject(ctx, a.ObjectKey)
	if err != nil {
		return s.failValidation(ctx, id)
	}
	if info.Size <= 0 || info.Size > maxImageBytes || info.Size != a.SizeBytes || imagevalidate.NormalizeContentType(info.ContentType) != imagevalidate.NormalizeContentType(a.MimeType) {
		return s.failValidation(ctx, id)
	}
	body, err := s.storage.OpenObject(ctx, a.ObjectKey)
	if err != nil {
		return s.failValidation(ctx, id)
	}
	defer body.Close()
	metadata, err := imagevalidate.Validate(body, a.MimeType, info.Size, imageConstraints)
	if err != nil {
		return s.failValidation(ctx, id)
	}
	if err := s.repo.MarkReady(ctx, id, metadata.ContentType, info.Size, metadata.Width, metadata.Height, now); err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return ErrMediaUploadExpired
		}
		return err
	}
	return nil
}

func (s *Service) CleanupExpiredUploads(ctx context.Context, batchSize int) (int, error) {
	if s.storage == nil {
		return 0, nil
	}
	now := time.Now().UTC()
	assets, err := s.repo.ClaimExpired(ctx, now, now.Add(-uploadCleanupLease), batchSize)
	if err != nil {
		return 0, err
	}
	cleaned := 0
	var firstErr error
	for _, asset := range assets {
		if err := s.storage.DeleteObject(ctx, asset.ObjectKey); err != nil {
			if recordErr := s.repo.RecordCleanupFailure(ctx, asset.ID, err.Error()); recordErr != nil && firstErr == nil {
				firstErr = recordErr
			}
			if firstErr == nil {
				firstErr = fmt.Errorf("delete expired media object %d: %w", asset.ID, err)
			}
			continue
		}
		if err := s.repo.MarkDeleted(ctx, asset.ID, now); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		cleaned++
	}
	return cleaned, firstErr
}

func (s *Service) failValidation(ctx context.Context, id uint) error {
	if err := s.repo.MarkFailed(ctx, id); err != nil {
		return err
	}
	return ErrMediaValidationFailed
}

func allowed(v string) bool {
	return imagevalidate.SupportsContentType(v, imageConstraints)
}
