//go:build integration

package cms

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	domainCMS "github.com/freeDog-wy/go-backend-template/internal/domain/cms"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	"github.com/freeDog-wy/go-backend-template/internal/infra/database"
	modelCMS "github.com/freeDog-wy/go-backend-template/internal/model/cms"
	"github.com/freeDog-wy/go-backend-template/internal/testsupport"
)

func TestRepositoryIntegrationCMSConstraintsAndPublicVisibility(t *testing.T) {
	ctx := context.Background()
	db := testsupport.OpenPostgres(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("database handle: %v", err)
	}
	migrator, err := database.NewMigratorWithDB(sqlDB, migrationDir(t))
	if err != nil {
		t.Fatalf("open migrator: %v", err)
	}
	t.Cleanup(func() { _, _ = migrator.Close() })
	if err := migrator.Up(); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	var authorID uint
	if err := db.Raw(`INSERT INTO users (name, email, created_at, updated_at) VALUES ('CMS Author', ?, NOW(), NOW()) RETURNING id`, fmt.Sprintf("cms-it-%d@example.com", time.Now().UnixNano())).Scan(&authorID).Error; err != nil {
		t.Fatalf("create author: %v", err)
	}
	repo := New(db)
	category, err := createCategory(ctx, repo, "root")
	if err != nil {
		t.Fatal(err)
	}

	article, draft := createArticle(ctx, repo, authorID, "draft-only")
	if _, err := repo.FindPublicArticle(ctx, "zh-CN", draft.Slug); !errors.Is(err, shared.ErrNotFound) {
		t.Fatalf("draft public lookup error = %v", err)
	}
	article2, translation2 := createArticle(ctx, repo, authorID, "published")
	now := time.Now().UTC()
	translation2.Status, translation2.PublishedAt = domainCMS.TranslationPublished, &now
	if err := repo.SaveArticleTranslation(ctx, translation2); err != nil {
		t.Fatalf("publish translation: %v", err)
	}
	if got, err := repo.FindPublicArticle(ctx, "zh-CN", translation2.Slug); err != nil || got.Article.ID != article2.ID {
		t.Fatalf("public article = %#v, %v", got, err)
	}
	if err := repo.ReplaceArticleCategories(ctx, article2.ID, []uint{category.ID}, &category.ID); err != nil {
		t.Fatalf("set published article category: %v", err)
	}
	publicArticles, total, err := repo.ListPublicArticles(ctx, "zh-CN", nil, shared.NewPageQuery(1, 20))
	if err != nil || total != 1 || len(publicArticles) != 1 || publicArticles[0].Article.ID != article2.ID {
		t.Fatalf("public list = %#v, total=%d, err=%v", publicArticles, total, err)
	}
	categorySlug := "root"
	categoryArticles, total, err := repo.ListPublicArticles(ctx, "zh-CN", &categorySlug, shared.NewPageQuery(1, 20))
	if err != nil || total != 1 || len(categoryArticles) != 1 || categoryArticles[0].Article.ID != article2.ID {
		t.Fatalf("category public list = %#v, total=%d, err=%v", categoryArticles, total, err)
	}
	disabled, err := createCategory(ctx, repo, "disabled")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&modelCMS.Category{}).Where("id = ?", disabled.ID).Update("is_enabled", false).Error; err != nil {
		t.Fatalf("disable category: %v", err)
	}
	publicCategories, err := repo.ListPublicCategoryTreeItems(ctx, "zh-CN")
	if err != nil || len(publicCategories) != 1 || publicCategories[0].ID != category.ID {
		t.Fatalf("public categories = %#v, err=%v", publicCategories, err)
	}

	if err := repo.ReplaceArticleCategories(ctx, article.ID, []uint{category.ID}, &category.ID); err != nil {
		t.Fatalf("set primary category: %v", err)
	}
	if err := db.Create(&modelCMS.ArticleCategory{ArticleID: article.ID, CategoryID: category.ID + 1000, IsPrimary: true}).Error; err == nil {
		t.Fatal("expected primary category constraint error")
	}
	duplicate := &domainCMS.ArticleTranslation{ArticleID: article2.ID, Locale: "zh-CN", Title: "Duplicate", Slug: draft.Slug, ContentFormat: "markdown", Status: domainCMS.TranslationDraft}
	if err := repo.CreateArticleTranslation(ctx, duplicate); err == nil {
		t.Fatal("expected locale slug uniqueness error")
	}
}

func createCategory(ctx context.Context, repo *Repository, slug string) (*domainCMS.Category, error) {
	category := &domainCMS.Category{Enabled: true}
	translation := &domainCMS.CategoryTranslation{Locale: "zh-CN", Name: slug, Slug: slug}
	return category, repo.CreateCategory(ctx, category, translation)
}

func createArticle(ctx context.Context, repo *Repository, authorID uint, slug string) (*domainCMS.Article, *domainCMS.ArticleTranslation) {
	article := &domainCMS.Article{AuthorUserID: authorID}
	translation := &domainCMS.ArticleTranslation{Locale: "zh-CN", Title: slug, Slug: slug, ContentFormat: "markdown", Status: domainCMS.TranslationDraft}
	if err := repo.CreateArticle(ctx, article, translation); err != nil {
		panic(err)
	}
	return article, translation
}

func migrationDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
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
