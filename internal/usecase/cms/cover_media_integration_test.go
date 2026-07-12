//go:build integration

package cms

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	appConfig "github.com/freeDog-wy/go-backend-template/internal/config"
	"github.com/freeDog-wy/go-backend-template/internal/infra/database"
	infraStorage "github.com/freeDog-wy/go-backend-template/internal/infra/storage"
	repoCMS "github.com/freeDog-wy/go-backend-template/internal/repository/cms"
	repoMedia "github.com/freeDog-wy/go-backend-template/internal/repository/media"
	"github.com/freeDog-wy/go-backend-template/internal/testsupport"
	svcMedia "github.com/freeDog-wy/go-backend-template/internal/usecase/media"
)

func TestCoverMediaIntegrationUploadCompleteAndPublish(t *testing.T) {
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
	if err := db.Raw(`INSERT INTO users (name, email, created_at, updated_at) VALUES ('Cover Author', ?, NOW(), NOW()) RETURNING id`, fmt.Sprintf("cover-media-it-%d@example.com", time.Now().UnixNano())).Scan(&userID).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	resource := testsupport.OpenS3(t)
	storage, err := infraStorage.NewS3(ctx, appConfig.S3Config{
		Endpoint:        resource.Endpoint,
		Region:          resource.Region,
		AccessKeyID:     resource.AccessKeyID,
		SecretAccessKey: resource.SecretAccessKey,
		Bucket:          resource.Bucket,
		PublicBaseURL:   "https://cdn.example.test",
		Prefix:          "cms",
		UsePathStyle:    true,
	})
	if err != nil {
		t.Fatalf("create S3 storage: %v", err)
	}

	tx := database.NewTxManager(db)
	mediaRepo := repoMedia.New(db)
	mediaSvc := svcMedia.New(tx, mediaRepo, storage)
	cmsSvc := New(tx, repoCMS.New(db))
	cmsSvc.SetMediaFinder(mediaSvc)
	cmsSvc.SetPublicMediaFinder(mediaSvc)

	article, err := cmsSvc.CreateArticle(ctx, CreateArticleCmd{
		AuthorUserID:  userID,
		Locale:        "zh-CN",
		Title:         "Cover media integration",
		Slug:          "cover-media-integration",
		Summary:       "Integration coverage for article covers.",
		Content:       "# Content",
		ContentFormat: "markdown",
	})
	if err != nil {
		t.Fatalf("create article: %v", err)
	}
	if _, err := cmsSvc.PublishTranslation(ctx, PublishTranslationCmd{ArticleID: article.ID, Locale: "zh-CN", ActorUserID: userID}); err != nil {
		t.Fatalf("publish article: %v", err)
	}

	payload := tinyPNG(t)
	upload, err := mediaSvc.RequestUpload(ctx, svcMedia.UploadRequest{Filename: "cover.png", ContentType: "image/png", SizeBytes: int64(len(payload)), UserID: userID})
	if err != nil {
		t.Fatalf("request upload: %v", err)
	}
	if err := cmsSvc.SetArticleCover(ctx, SetArticleCoverCmd{ArticleID: article.ID, MediaID: &upload.ID, ActorUserID: userID}); err == nil {
		t.Fatal("set pending media as cover succeeded")
	}

	if err := putPresignedUpload(ctx, upload.UploadURL, upload.Headers, payload); err != nil {
		t.Fatalf("upload cover to S3: %v", err)
	}
	if err := mediaSvc.Complete(ctx, upload.ID, userID); err != nil {
		t.Fatalf("complete media upload: %v", err)
	}
	asset, err := mediaRepo.Find(ctx, upload.ID)
	if err != nil {
		t.Fatalf("find completed media: %v", err)
	}
	if asset.Status != "ready" || asset.SizeBytes != int64(len(payload)) || asset.Width != 2 || asset.Height != 3 {
		t.Fatalf("completed media = %#v", asset)
	}
	if err := mediaSvc.UpsertTranslation(ctx, upload.ID, "zh-CN", "Article cover", "Cover title"); err != nil {
		t.Fatalf("translate media: %v", err)
	}
	if err := cmsSvc.SetArticleCover(ctx, SetArticleCoverCmd{ArticleID: article.ID, MediaID: &upload.ID, ActorUserID: userID}); err != nil {
		t.Fatalf("set ready media as cover: %v", err)
	}

	publicArticle, err := cmsSvc.GetPublishedArticle(ctx, "zh-CN", "cover-media-integration")
	if err != nil {
		t.Fatalf("get published article: %v", err)
	}
	if publicArticle.Cover == nil {
		t.Fatal("published article cover is nil")
	}
	if publicArticle.Cover.ID != upload.ID || publicArticle.Cover.URL != "https://cdn.example.test/"+upload.ObjectKey || publicArticle.Cover.AltText != "Article cover" || publicArticle.Cover.Title != "Cover title" {
		t.Fatalf("published article cover = %#v", publicArticle.Cover)
	}

	spoofedPayload := []byte("not an image")
	spoofed, err := mediaSvc.RequestUpload(ctx, svcMedia.UploadRequest{Filename: "spoofed.png", ContentType: "image/png", SizeBytes: int64(len(spoofedPayload)), UserID: userID})
	if err != nil {
		t.Fatalf("request spoofed upload: %v", err)
	}
	if err := putPresignedUpload(ctx, spoofed.UploadURL, spoofed.Headers, spoofedPayload); err != nil {
		t.Fatalf("upload spoofed object: %v", err)
	}
	if err := mediaSvc.Complete(ctx, spoofed.ID, userID); !errors.Is(err, svcMedia.ErrMediaValidationFailed) {
		t.Fatalf("complete spoofed object error = %v, want validation failure", err)
	}
	spoofedAsset, err := mediaRepo.Find(ctx, spoofed.ID)
	if err != nil {
		t.Fatalf("find spoofed media: %v", err)
	}
	if spoofedAsset.Status != "failed" {
		t.Fatalf("spoofed media status = %q, want failed", spoofedAsset.Status)
	}
	if err := cmsSvc.SetArticleCover(ctx, SetArticleCoverCmd{ArticleID: article.ID, MediaID: &spoofed.ID, ActorUserID: userID}); err == nil {
		t.Fatal("set failed media as cover succeeded")
	}
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	imageData := image.NewRGBA(image.Rect(0, 0, 2, 3))
	imageData.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, imageData); err != nil {
		t.Fatalf("encode PNG fixture: %v", err)
	}
	return buffer.Bytes()
}

func putPresignedUpload(ctx context.Context, url string, headers map[string]string, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("upload response status %d", response.StatusCode)
	}
	return nil
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
