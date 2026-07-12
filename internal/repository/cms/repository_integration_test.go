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
	infraOutbox "github.com/freeDog-wy/go-backend-template/internal/infra/outbox"
	modelCMS "github.com/freeDog-wy/go-backend-template/internal/model/cms"
	repoOutbox "github.com/freeDog-wy/go-backend-template/internal/repository/outbox"
	"github.com/freeDog-wy/go-backend-template/internal/testsupport"
	svcCMS "github.com/freeDog-wy/go-backend-template/internal/usecase/cms"
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
	enLocale := &domainCMS.Locale{Code: "en-US", Name: "English", IsEnabled: true, SortOrder: 1}
	if err := repo.CreateLocale(ctx, enLocale); err != nil {
		t.Fatalf("create locale: %v", err)
	}
	if err := repo.SetDefaultLocale(ctx, enLocale.Code); err != nil {
		t.Fatalf("set default locale: %v", err)
	}
	if locale, err := repo.FindLocale(ctx, "zh-CN"); err != nil || locale.IsDefault {
		t.Fatalf("old default locale = %#v, %v", locale, err)
	}
	if locale, err := repo.FindLocale(ctx, "en-US"); err != nil || !locale.IsDefault {
		t.Fatalf("new default locale = %#v, %v", locale, err)
	}
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
	tag := &domainCMS.Tag{}
	if err := repo.CreateTag(ctx, tag, &domainCMS.TagTranslation{Locale: "zh-CN", Name: "Go", Slug: "go"}); err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if err := repo.UpsertTagTranslation(ctx, &domainCMS.TagTranslation{TagID: tag.ID, Locale: "en-US", Name: "Go", Slug: "go"}); err != nil {
		t.Fatalf("translate tag: %v", err)
	}
	if err := repo.ReplaceArticleTags(ctx, article2.ID, []uint{tag.ID}); err != nil {
		t.Fatalf("attach tag: %v", err)
	}
	if tagged, total, err := repo.ListPublicTagArticles(ctx, "zh-CN", "go", shared.NewPageQuery(1, 20)); err != nil || total != 1 || len(tagged) != 1 || tagged[0].Article.ID != article2.ID {
		t.Fatalf("tag articles=%#v total=%d err=%v", tagged, total, err)
	}
	if err := db.Create(&modelCMS.ArticleTag{ArticleID: article2.ID, TagID: tag.ID}).Error; err == nil {
		t.Fatal("expected duplicate article tag constraint")
	}
	if err := repo.UpsertCategoryTranslation(ctx, &domainCMS.CategoryTranslation{CategoryID: category.ID, Locale: "en-US", Name: "Root", Slug: "root", Description: "English root"}); err != nil {
		t.Fatalf("upsert category translation: %v", err)
	}
	enTranslation := &domainCMS.ArticleTranslation{ArticleID: article2.ID, Locale: "en-US", Title: "Published", Slug: "published", ContentFormat: "markdown", Status: domainCMS.TranslationPublished, PublishedAt: &now}
	if err := repo.CreateArticleTranslation(ctx, enTranslation); err != nil {
		t.Fatalf("create english translation: %v", err)
	}
	if categories, err := repo.ListCategoryTreeItems(ctx, "en-US"); err != nil || len(categories) != 1 || categories[0].Name != "Root" {
		t.Fatalf("english categories = %#v, %v", categories, err)
	}
	if locales, err := repo.ListPublishedArticleLocales(ctx, article2.ID); err != nil || len(locales) != 2 {
		t.Fatalf("published locales = %#v, %v", locales, err)
	}
	if breadcrumbs, err := repo.ListPublicArticleBreadcrumbs(ctx, article2.ID, "zh-CN"); err != nil || len(breadcrumbs) != 1 || breadcrumbs[0].Slug != "root" {
		t.Fatalf("breadcrumbs = %#v, %v", breadcrumbs, err)
	}
	if entries, total, err := repo.ListPublicSitemapEntries(ctx, "zh-CN", shared.NewPageQuery(1, 20)); err != nil || total < 2 || len(entries) < 2 {
		t.Fatalf("sitemap entries = %#v, total=%d, err=%v", entries, total, err)
	}
	redirectService := svcCMS.New(database.NewTxManager(db), repo)
	oldArticleSlug := translation2.Slug
	if _, err := redirectService.UpdateTranslation(ctx, svcCMS.UpdateTranslationCmd{ArticleID: article2.ID, Locale: "zh-CN", Title: translation2.Title, Slug: "published-renamed", Summary: translation2.Summary, Content: translation2.Content, ContentFormat: translation2.ContentFormat, SEOTitle: translation2.SEOTitle, SEODescription: translation2.SEODescription, CanonicalURL: translation2.CanonicalURL}); err != nil {
		t.Fatalf("rename article slug: %v", err)
	}
	if _, err := redirectService.UpdateTranslation(ctx, svcCMS.UpdateTranslationCmd{ArticleID: article2.ID, Locale: "zh-CN", Title: translation2.Title, Slug: "published-final", Summary: translation2.Summary, Content: translation2.Content, ContentFormat: translation2.ContentFormat, SEOTitle: translation2.SEOTitle, SEODescription: translation2.SEODescription, CanonicalURL: translation2.CanonicalURL}); err != nil {
		t.Fatalf("rename article slug again: %v", err)
	}
	translation2.Slug = "published-final"
	if redirect, err := repo.FindURLRedirect(ctx, "zh-CN", "/zh-CN/articles/"+oldArticleSlug); err != nil || redirect.TargetPath != "/zh-CN/articles/published-final" {
		t.Fatalf("article redirect = %#v, %v", redirect, err)
	}
	if _, err := redirectService.UpsertCategoryTranslation(ctx, svcCMS.UpsertCategoryTranslationCmd{CategoryID: category.ID, Locale: "zh-CN", Name: "Root", Slug: "root-renamed"}); err != nil {
		t.Fatalf("rename category slug: %v", err)
	}
	if redirect, err := repo.FindURLRedirect(ctx, "zh-CN", "/zh-CN/categories/root"); err != nil || redirect.TargetPath != "/zh-CN/categories/root-renamed" {
		t.Fatalf("category redirect = %#v, %v", redirect, err)
	}
	publicArticles, total, err := repo.ListPublicArticles(ctx, "zh-CN", nil, shared.NewPageQuery(1, 20))
	if err != nil || total != 1 || len(publicArticles) != 1 || publicArticles[0].Article.ID != article2.ID {
		t.Fatalf("public list = %#v, total=%d, err=%v", publicArticles, total, err)
	}
	categorySlug := "root-renamed"
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

	service := svcCMS.New(database.NewTxManager(db), repo, infraOutbox.NewEventBus(repoOutbox.New(db)))
	if err := service.DeleteArticle(ctx, svcCMS.DeleteArticleCmd{ArticleID: article2.ID, ActorUserID: authorID}); err != nil {
		t.Fatalf("delete article: %v", err)
	}
	if _, err := repo.FindPublicArticle(ctx, "zh-CN", translation2.Slug); !errors.Is(err, shared.ErrNotFound) {
		t.Fatalf("deleted public lookup error = %v", err)
	}
	var auditOutboxCount int64
	if err := db.Table("outbox_events").Where("event_name = ?", "audit.log.requested").Count(&auditOutboxCount).Error; err != nil || auditOutboxCount != 1 {
		t.Fatalf("audit outbox count = %d, err=%v", auditOutboxCount, err)
	}
	if err := service.RestoreArticle(ctx, svcCMS.RestoreArticleCmd{ArticleID: article2.ID, ActorUserID: authorID}); err != nil {
		t.Fatalf("restore article: %v", err)
	}
	if got, err := repo.FindPublicArticle(ctx, "zh-CN", translation2.Slug); err != nil || got.Article.ID != article2.ID {
		t.Fatalf("restored public article = %#v, %v", got, err)
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
