package cms

import (
	"context"
	"errors"
	"time"

	domainCMS "github.com/freeDog-wy/go-backend-template/internal/domain/cms"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	"github.com/freeDog-wy/go-backend-template/internal/infra/database"
	modelCMS "github.com/freeDog-wy/go-backend-template/internal/model/cms"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

var _ domainCMS.Repository = (*Repository)(nil)

func New(db *gorm.DB) *Repository                       { return &Repository{db: db} }
func (r *Repository) conn(ctx context.Context) *gorm.DB { return database.DB(ctx, r.db) }

func (r *Repository) LocaleEnabled(ctx context.Context, code string) (bool, error) {
	var count int64
	err := r.conn(ctx).Table("locales").Where("code = ? AND is_enabled", code).Count(&count).Error
	return count == 1, err
}

func (r *Repository) CreateCategory(ctx context.Context, category *domainCMS.Category, tr *domainCMS.CategoryTranslation) error {
	m := modelCMS.Category{ParentID: category.ParentID, SortOrder: category.SortOrder, IsEnabled: category.Enabled}
	if err := r.conn(ctx).Create(&m).Error; err != nil {
		return err
	}
	category.ID, category.CreatedAt, category.UpdatedAt = m.ID, m.CreatedAt, m.UpdatedAt
	return r.conn(ctx).Create(&modelCMS.CategoryTranslation{CategoryID: m.ID, Locale: tr.Locale, Name: tr.Name, Slug: tr.Slug, Description: tr.Description, SEOTitle: tr.SEOTitle, SEODescription: tr.SEODescription}).Error
}

func (r *Repository) FindCategory(ctx context.Context, id uint) (*domainCMS.Category, error) {
	var m modelCMS.Category
	if err := r.conn(ctx).First(&m, id).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &domainCMS.Category{ID: m.ID, ParentID: m.ParentID, SortOrder: m.SortOrder, Enabled: m.IsEnabled, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}, nil
}

func (r *Repository) IsCategoryDescendant(ctx context.Context, ancestorID, candidateID uint) (bool, error) {
	var count int64
	err := r.conn(ctx).Raw(`WITH RECURSIVE descendants AS (
 SELECT id FROM categories WHERE parent_id = ?
 UNION ALL SELECT c.id FROM categories c JOIN descendants d ON c.parent_id = d.id
) SELECT COUNT(*) FROM descendants WHERE id = ?`, ancestorID, candidateID).Scan(&count).Error
	return count > 0, err
}

func (r *Repository) MoveCategory(ctx context.Context, id uint, parentID *uint, sortOrder int) error {
	result := r.conn(ctx).Model(&modelCMS.Category{}).Where("id = ?", id).Updates(map[string]any{"parent_id": parentID, "sort_order": sortOrder})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (r *Repository) ListCategories(ctx context.Context) ([]*domainCMS.Category, error) {
	var models []modelCMS.Category
	if err := r.conn(ctx).Order("parent_id NULLS FIRST, sort_order, id").Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]*domainCMS.Category, 0, len(models))
	for _, m := range models {
		result = append(result, &domainCMS.Category{ID: m.ID, ParentID: m.ParentID, SortOrder: m.SortOrder, Enabled: m.IsEnabled, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt})
	}
	return result, nil
}

func (r *Repository) ListCategoryTreeItems(ctx context.Context, locale string) ([]*domainCMS.CategoryTreeItem, error) {
	return r.listCategoryTreeItems(ctx, locale, false)
}

func (r *Repository) ListPublicCategoryTreeItems(ctx context.Context, locale string) ([]*domainCMS.CategoryTreeItem, error) {
	return r.listCategoryTreeItems(ctx, locale, true)
}

func (r *Repository) listCategoryTreeItems(ctx context.Context, locale string, enabledOnly bool) ([]*domainCMS.CategoryTreeItem, error) {
	type row struct {
		CategoryID     uint
		ParentID       *uint
		SortOrder      int
		IsEnabled      bool
		Name           string
		Slug           string
		Description    string
		SEOTitle       string
		SEODescription string
	}
	var rows []row
	db := r.conn(ctx).Table("categories").
		Select("categories.id AS category_id, categories.parent_id, categories.sort_order, categories.is_enabled, category_translations.name, category_translations.slug, category_translations.description, category_translations.seo_title, category_translations.seo_description").
		Joins("JOIN category_translations ON category_translations.category_id = categories.id").Where("category_translations.locale = ?", locale)
	if enabledOnly {
		db = db.Where("categories.is_enabled")
	}
	err := db.Order("categories.parent_id NULLS FIRST, categories.sort_order, categories.id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	items := make([]*domainCMS.CategoryTreeItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, &domainCMS.CategoryTreeItem{Category: domainCMS.Category{ID: row.CategoryID, ParentID: row.ParentID, SortOrder: row.SortOrder, Enabled: row.IsEnabled}, CategoryTranslation: domainCMS.CategoryTranslation{CategoryID: row.CategoryID, Locale: locale, Name: row.Name, Slug: row.Slug, Description: row.Description, SEOTitle: row.SEOTitle, SEODescription: row.SEODescription}})
	}
	return items, nil
}

func (r *Repository) CreateArticle(ctx context.Context, article *domainCMS.Article, tr *domainCMS.ArticleTranslation) error {
	m := modelCMS.Article{AuthorUserID: article.AuthorUserID}
	if err := r.conn(ctx).Create(&m).Error; err != nil {
		return err
	}
	article.ID, article.CreatedAt, article.UpdatedAt = m.ID, m.CreatedAt, m.UpdatedAt
	tm := translationModel(m.ID, tr)
	if err := r.conn(ctx).Create(&tm).Error; err != nil {
		return err
	}
	tr.ID, tr.ArticleID, tr.CreatedAt, tr.UpdatedAt = tm.ID, m.ID, tm.CreatedAt, tm.UpdatedAt
	return nil
}

func (r *Repository) FindArticle(ctx context.Context, id uint) (*domainCMS.Article, error) {
	var m modelCMS.Article
	if err := r.conn(ctx).Where("deleted_at IS NULL").First(&m, id).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &domainCMS.Article{ID: m.ID, AuthorUserID: m.AuthorUserID, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, DeletedAt: m.DeletedAt}, nil
}

func (r *Repository) FindArticleTranslation(ctx context.Context, articleID uint, locale string) (*domainCMS.ArticleTranslation, error) {
	var m modelCMS.ArticleTranslation
	if err := r.conn(ctx).Where("article_id = ? AND locale = ?", articleID, locale).First(&m).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return translationEntity(m), nil
}

func (r *Repository) CreateArticleTranslation(ctx context.Context, tr *domainCMS.ArticleTranslation) error {
	m := translationModel(tr.ArticleID, tr)
	if err := r.conn(ctx).Create(&m).Error; err != nil {
		return err
	}
	tr.ID, tr.CreatedAt, tr.UpdatedAt = m.ID, m.CreatedAt, m.UpdatedAt
	return nil
}

func (r *Repository) SaveArticleTranslation(ctx context.Context, tr *domainCMS.ArticleTranslation) error {
	m := translationModel(tr.ArticleID, tr)
	result := r.conn(ctx).Model(&modelCMS.ArticleTranslation{}).Where("id = ?", tr.ID).Updates(map[string]any{"title": m.Title, "slug": m.Slug, "summary": m.Summary, "content": m.Content, "content_format": m.ContentFormat, "status": m.Status, "published_at": m.PublishedAt, "seo_title": m.SEOTitle, "seo_description": m.SEODescription, "canonical_url": m.CanonicalURL})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (r *Repository) ReplaceArticleCategories(ctx context.Context, articleID uint, categoryIDs []uint, primaryCategoryID *uint) error {
	db := r.conn(ctx)
	if err := db.Where("article_id = ?", articleID).Delete(&modelCMS.ArticleCategory{}).Error; err != nil {
		return err
	}
	if len(categoryIDs) == 0 {
		return nil
	}
	records := make([]modelCMS.ArticleCategory, 0, len(categoryIDs))
	for _, categoryID := range categoryIDs {
		records = append(records, modelCMS.ArticleCategory{ArticleID: articleID, CategoryID: categoryID, IsPrimary: primaryCategoryID != nil && *primaryCategoryID == categoryID})
	}
	return db.Create(&records).Error
}

func (r *Repository) ListArticleTranslations(ctx context.Context, locale string, page shared.PageQuery) ([]*domainCMS.ArticleListItem, int64, error) {
	db := r.conn(ctx).Table("article_translations").Joins("JOIN articles ON articles.id = article_translations.article_id").Where("article_translations.locale = ? AND articles.deleted_at IS NULL", locale)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	type row struct {
		ArticleID                                                                                    uint
		AuthorUserID                                                                                 uint
		ArticleCreatedAt, ArticleUpdatedAt                                                           time.Time
		TranslationID                                                                                uint
		Title, Slug, Summary, Content, ContentFormat, Status, SEOTitle, SEODescription, CanonicalURL string
		PublishedAt                                                                                  *time.Time
		TranslationCreatedAt, TranslationUpdatedAt                                                   time.Time
	}
	var rows []row
	err := db.Select("articles.id AS article_id, articles.author_user_id, articles.created_at AS article_created_at, articles.updated_at AS article_updated_at, article_translations.id AS translation_id, article_translations.title, article_translations.slug, article_translations.summary, article_translations.content, article_translations.content_format, article_translations.status, article_translations.published_at, article_translations.seo_title, article_translations.seo_description, article_translations.canonical_url, article_translations.created_at AS translation_created_at, article_translations.updated_at AS translation_updated_at").Order("article_translations.updated_at DESC, article_translations.id DESC").Limit(page.PerPage).Offset(page.Offset()).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	items := make([]*domainCMS.ArticleListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, &domainCMS.ArticleListItem{Article: domainCMS.Article{ID: row.ArticleID, AuthorUserID: row.AuthorUserID, CreatedAt: row.ArticleCreatedAt, UpdatedAt: row.ArticleUpdatedAt}, ArticleTranslation: domainCMS.ArticleTranslation{ID: row.TranslationID, ArticleID: row.ArticleID, Locale: locale, Title: row.Title, Slug: row.Slug, Summary: row.Summary, Content: row.Content, ContentFormat: row.ContentFormat, Status: domainCMS.TranslationStatus(row.Status), PublishedAt: row.PublishedAt, SEOTitle: row.SEOTitle, SEODescription: row.SEODescription, CanonicalURL: row.CanonicalURL, CreatedAt: row.TranslationCreatedAt, UpdatedAt: row.TranslationUpdatedAt}})
	}
	return items, total, nil
}

func (r *Repository) FindPublicArticle(ctx context.Context, locale, slug string) (*domainCMS.PublicArticle, error) {
	var article modelCMS.Article
	err := r.conn(ctx).Table("articles").Select("articles.*").Joins("JOIN article_translations ON article_translations.article_id = articles.id").Where("articles.deleted_at IS NULL AND article_translations.locale = ? AND article_translations.slug = ? AND article_translations.status = 'published' AND article_translations.published_at <= NOW()", locale, slug).First(&article).Error
	if err != nil {
		return nil, mapNotFound(err)
	}
	tr, err := r.FindArticleTranslation(ctx, article.ID, locale)
	if err != nil {
		return nil, err
	}
	return &domainCMS.PublicArticle{Article: domainCMS.Article{ID: article.ID, AuthorUserID: article.AuthorUserID, CreatedAt: article.CreatedAt, UpdatedAt: article.UpdatedAt, DeletedAt: article.DeletedAt}, ArticleTranslation: *tr}, nil
}

func (r *Repository) PublicCategoryExists(ctx context.Context, locale, slug string) (bool, error) {
	var count int64
	err := r.conn(ctx).Table("categories").Joins("JOIN category_translations ON category_translations.category_id = categories.id").Where("categories.is_enabled AND category_translations.locale = ? AND category_translations.slug = ?", locale, slug).Count(&count).Error
	return count == 1, err
}

func (r *Repository) ListPublicArticles(ctx context.Context, locale string, categorySlug *string, page shared.PageQuery) ([]*domainCMS.PublicArticleListItem, int64, error) {
	db := r.conn(ctx).Table("article_translations").
		Joins("JOIN articles ON articles.id = article_translations.article_id").
		Joins("LEFT JOIN article_categories primary_ac ON primary_ac.article_id = articles.id AND primary_ac.is_primary").
		Joins("LEFT JOIN categories primary_c ON primary_c.id = primary_ac.category_id AND primary_c.is_enabled").
		Joins("LEFT JOIN category_translations primary_ct ON primary_ct.category_id = primary_c.id AND primary_ct.locale = article_translations.locale").
		Where("articles.deleted_at IS NULL AND article_translations.locale = ? AND article_translations.status = 'published' AND article_translations.published_at <= NOW()", locale)
	if categorySlug != nil {
		db = db.Joins("JOIN article_categories filter_ac ON filter_ac.article_id = articles.id").
			Joins("JOIN categories filter_c ON filter_c.id = filter_ac.category_id AND filter_c.is_enabled").
			Joins("JOIN category_translations filter_ct ON filter_ct.category_id = filter_c.id AND filter_ct.locale = article_translations.locale").
			Where("filter_ct.slug = ?", *categorySlug)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	type row struct {
		ArticleID                                uint
		Title, Slug, Summary, ContentFormat      string
		PublishedAt                              *time.Time
		UpdatedAt                                time.Time
		PrimaryCategoryID                        *uint
		PrimaryCategoryName, PrimaryCategorySlug string
	}
	var rows []row
	err := db.Select("articles.id AS article_id, article_translations.title, article_translations.slug, article_translations.summary, article_translations.content_format, article_translations.published_at, article_translations.updated_at, primary_c.id AS primary_category_id, primary_ct.name AS primary_category_name, primary_ct.slug AS primary_category_slug").Order("article_translations.published_at DESC, article_translations.id DESC").Limit(page.PerPage).Offset(page.Offset()).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	items := make([]*domainCMS.PublicArticleListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, &domainCMS.PublicArticleListItem{Article: domainCMS.Article{ID: row.ArticleID, UpdatedAt: row.UpdatedAt}, ArticleTranslation: domainCMS.ArticleTranslation{ArticleID: row.ArticleID, Locale: locale, Title: row.Title, Slug: row.Slug, Summary: row.Summary, ContentFormat: row.ContentFormat, PublishedAt: row.PublishedAt, Status: domainCMS.TranslationPublished, UpdatedAt: row.UpdatedAt}, PrimaryCategoryID: row.PrimaryCategoryID, PrimaryCategoryName: row.PrimaryCategoryName, PrimaryCategorySlug: row.PrimaryCategorySlug})
	}
	return items, total, nil
}

func translationModel(articleID uint, tr *domainCMS.ArticleTranslation) modelCMS.ArticleTranslation {
	return modelCMS.ArticleTranslation{ID: tr.ID, ArticleID: articleID, Locale: tr.Locale, Title: tr.Title, Slug: tr.Slug, Summary: tr.Summary, Content: tr.Content, ContentFormat: tr.ContentFormat, Status: string(tr.Status), PublishedAt: tr.PublishedAt, SEOTitle: tr.SEOTitle, SEODescription: tr.SEODescription, CanonicalURL: tr.CanonicalURL}
}
func translationEntity(m modelCMS.ArticleTranslation) *domainCMS.ArticleTranslation {
	return &domainCMS.ArticleTranslation{ID: m.ID, ArticleID: m.ArticleID, Locale: m.Locale, Title: m.Title, Slug: m.Slug, Summary: m.Summary, Content: m.Content, ContentFormat: m.ContentFormat, Status: domainCMS.TranslationStatus(m.Status), PublishedAt: m.PublishedAt, SEOTitle: m.SEOTitle, SEODescription: m.SEODescription, CanonicalURL: m.CanonicalURL, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
}
func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return shared.ErrNotFound
	}
	return err
}
