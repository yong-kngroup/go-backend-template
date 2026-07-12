//go:build integration

package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/infra/database"
	modelCMS "github.com/freeDog-wy/go-backend-template/internal/model/cms"
	modelMedia "github.com/freeDog-wy/go-backend-template/internal/model/media"
	"github.com/freeDog-wy/go-backend-template/internal/testsupport"
)

func TestRepositoryIntegrationListReadyPublic(t *testing.T) {
	ctx := context.Background()
	db := testsupport.OpenPostgres(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	migrator, err := database.NewMigratorWithDB(sqlDB, migrationDir(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = migrator.Close() })
	if err := migrator.Up(); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	var userID uint
	if err := db.Raw(`INSERT INTO users (name, email, created_at, updated_at) VALUES ('Media User', ?, NOW(), NOW()) RETURNING id`, fmt.Sprintf("media-it-%d@example.com", time.Now().UnixNano())).Scan(&userID).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&modelCMS.Locale{Code: "en-US", Name: "English", IsEnabled: true}).Error; err != nil {
		t.Fatalf("create locale: %v", err)
	}
	now := time.Now().UTC()
	ready := &modelMedia.Asset{UploaderUserID: userID, ObjectKey: "cms/ready.png", OriginalFilename: "ready.png", MimeType: "image/png", SizeBytes: 10, Status: "ready"}
	pending := &modelMedia.Asset{UploaderUserID: userID, ObjectKey: "cms/pending.png", OriginalFilename: "pending.png", MimeType: "image/png", SizeBytes: 10, Status: "pending"}
	expiredAt := now.Add(-time.Minute)
	expired := &modelMedia.Asset{UploaderUserID: userID, ObjectKey: "cms/expired.png", OriginalFilename: "expired.png", MimeType: "image/png", SizeBytes: 10, Status: "pending", UploadExpiresAt: &expiredAt}
	futureExpiry := now.Add(time.Hour)
	futurePending := &modelMedia.Asset{UploaderUserID: userID, ObjectKey: "cms/future.png", OriginalFilename: "future.png", MimeType: "image/png", SizeBytes: 10, Status: "pending", UploadExpiresAt: &futureExpiry}
	deleted := &modelMedia.Asset{UploaderUserID: userID, ObjectKey: "cms/deleted.png", OriginalFilename: "deleted.png", MimeType: "image/png", SizeBytes: 10, Status: "ready", DeletedAt: &now}
	for _, asset := range []*modelMedia.Asset{ready, pending, expired, futurePending, deleted} {
		if err := db.Create(asset).Error; err != nil {
			t.Fatalf("create media asset: %v", err)
		}
	}
	if err := db.Create(&modelMedia.Translation{MediaID: ready.ID, Locale: "en-US", AltText: "Ready cover", Title: "Ready title"}).Error; err != nil {
		t.Fatalf("create media translation: %v", err)
	}

	assets, err := New(db).ListReadyPublic(ctx, "en-US", []uint{ready.ID, pending.ID, deleted.ID})
	if err != nil {
		t.Fatalf("list public media: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("public assets = %#v, want only ready asset", assets)
	}
	asset := assets[0]
	if asset.ID != ready.ID || asset.ObjectKey != "cms/ready.png" || asset.AltText != "Ready cover" || asset.Title != "Ready title" {
		t.Fatalf("public asset = %#v", asset)
	}

	repo := New(db)
	claimed, err := repo.ClaimExpired(ctx, now, now.Add(-time.Minute), 10)
	if err != nil {
		t.Fatalf("claim expired media: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != expired.ID {
		t.Fatalf("claimed media = %#v", claimed)
	}
	var expiredAfterClaim modelMedia.Asset
	if err := db.First(&expiredAfterClaim, expired.ID).Error; err != nil {
		t.Fatalf("find claimed media: %v", err)
	}
	if expiredAfterClaim.Status != "expired" {
		t.Fatalf("claimed media status = %q, want expired", expiredAfterClaim.Status)
	}
	if err := repo.RecordCleanupFailure(ctx, expired.ID, "temporary S3 error"); err != nil {
		t.Fatalf("record cleanup failure: %v", err)
	}
	reclaimed, err := repo.ClaimExpired(ctx, now, now.Add(-time.Minute), 10)
	if err != nil {
		t.Fatalf("reclaim expired media: %v", err)
	}
	if len(reclaimed) != 1 || reclaimed[0].ID != expired.ID {
		t.Fatalf("reclaimed media = %#v", reclaimed)
	}
}

func migrationDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(dir, "db", "migrations")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("locate db/migrations from the working directory")
		}
		dir = parent
	}
}
