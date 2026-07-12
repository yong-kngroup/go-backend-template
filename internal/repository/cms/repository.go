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
	"gorm.io/gorm/clause"
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

func (r *Repository) ListLocales(ctx context.Context) ([]*domainCMS.Locale, error) {
	var models []modelCMS.Locale
	if err := r.conn(ctx).Order("sort_order, code").Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]*domainCMS.Locale, 0, len(models))
	for _, m := range models {
		result = append(result, localeEntity(m))
	}
	return result, nil
}
func (r *Repository) FindLocale(ctx context.Context, code string) (*domainCMS.Locale, error) {
	var m modelCMS.Locale
	if err := r.conn(ctx).First(&m, "code = ?", code).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return localeEntity(m), nil
}
func (r *Repository) CreateLocale(ctx context.Context, locale *domainCMS.Locale) error {
	m := localeModel(locale)
	if err := r.conn(ctx).Create(&m).Error; err != nil {
		return err
	}
	locale.CreatedAt, locale.UpdatedAt = m.CreatedAt, m.UpdatedAt
	return nil
}
func (r *Repository) UpdateLocale(ctx context.Context, locale *domainCMS.Locale) error {
	m := localeModel(locale)
	result := r.conn(ctx).Model(&modelCMS.Locale{}).Where("code = ?", m.Code).Updates(map[string]any{"name": m.Name, "is_enabled": m.IsEnabled, "sort_order": m.SortOrder})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}
func (r *Repository) SetDefaultLocale(ctx context.Context, code string) error {
	db := r.conn(ctx)
	if err := db.Model(&modelCMS.Locale{}).Where("is_default").Update("is_default", false).Error; err != nil {
		return err
	}
	result := db.Model(&modelCMS.Locale{}).Where("code = ? AND is_enabled", code).Update("is_default", true)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}
func (r *Repository) CountEnabledLocales(ctx context.Context) (int64, error) {
	var count int64
	err := r.conn(ctx).Model(&modelCMS.Locale{}).Where("is_enabled").Count(&count).Error
	return count, err
}

func (r *Repository) CreateTag(ctx context.Context, tag *domainCMS.Tag, translation *domainCMS.TagTranslation) error {
	m := modelCMS.Tag{}
	if err := r.conn(ctx).Create(&m).Error; err != nil {
		return err
	}
	tag.ID, tag.CreatedAt, tag.UpdatedAt = m.ID, m.CreatedAt, m.UpdatedAt
	return r.conn(ctx).Create(&modelCMS.TagTranslation{TagID: m.ID, Locale: translation.Locale, Name: translation.Name, Slug: translation.Slug}).Error
}
func (r *Repository) FindTag(ctx context.Context, id uint) (*domainCMS.Tag, error) {
	var m modelCMS.Tag
	if err := r.conn(ctx).First(&m, id).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &domainCMS.Tag{ID: m.ID, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}, nil
}
func (r *Repository) FindTagTranslation(ctx context.Context, tagID uint, locale string) (*domainCMS.TagTranslation, error) {
	var m modelCMS.TagTranslation
	if err := r.conn(ctx).Where("tag_id = ? AND locale = ?", tagID, locale).First(&m).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &domainCMS.TagTranslation{TagID: m.TagID, Locale: m.Locale, Name: m.Name, Slug: m.Slug}, nil
}
func (r *Repository) UpsertTagTranslation(ctx context.Context, translation *domainCMS.TagTranslation) error {
	m := modelCMS.TagTranslation{TagID: translation.TagID, Locale: translation.Locale, Name: translation.Name, Slug: translation.Slug}
	return r.conn(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tag_id"}, {Name: "locale"}}, DoUpdates: clause.Assignments(map[string]any{"name": m.Name, "slug": m.Slug, "updated_at": time.Now()})}).Create(&m).Error
}
func (r *Repository) ListTags(ctx context.Context, locale string, page shared.PageQuery) ([]*domainCMS.TagListItem, int64, error) {
	db := r.conn(ctx).Table("tag_translations").Joins("JOIN tags ON tags.id = tag_translations.tag_id").Where("tag_translations.locale = ?", locale)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	type row struct {
		TagID                uint
		Name, Slug           string
		CreatedAt, UpdatedAt time.Time
	}
	var rows []row
	if err := db.Select("tags.id AS tag_id, tags.created_at, tags.updated_at, tag_translations.name, tag_translations.slug").Order("tag_translations.name, tags.id").Limit(page.PerPage).Offset(page.Offset()).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	result := make([]*domainCMS.TagListItem, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domainCMS.TagListItem{Tag: domainCMS.Tag{ID: row.TagID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, TagTranslation: domainCMS.TagTranslation{TagID: row.TagID, Locale: locale, Name: row.Name, Slug: row.Slug}})
	}
	return result, total, nil
}

func (r *Repository) CreateCategory(ctx context.Context, category *domainCMS.Category, tr *domainCMS.CategoryTranslation) error {
	m := modelCMS.Category{ParentID: category.ParentID, SortOrder: category.SortOrder, IsEnabled: category.Enabled}
	if err := r.conn(ctx).Create(&m).Error; err != nil {
		return err
	}
	category.ID, category.CreatedAt, category.UpdatedAt = m.ID, m.CreatedAt, m.UpdatedAt
	return r.conn(ctx).Create(&modelCMS.CategoryTranslation{CategoryID: m.ID, Locale: tr.Locale, Name: tr.Name, Slug: tr.Slug, Description: tr.Description, SEOTitle: tr.SEOTitle, SEODescription: tr.SEODescription}).Error
}

func (r *Repository) UpsertCategoryTranslation(ctx context.Context, tr *domainCMS.CategoryTranslation) error {
	m := modelCMS.CategoryTranslation{CategoryID: tr.CategoryID, Locale: tr.Locale, Name: tr.Name, Slug: tr.Slug, Description: tr.Description, SEOTitle: tr.SEOTitle, SEODescription: tr.SEODescription}
	return r.conn(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "category_id"}, {Name: "locale"}}, DoUpdates: clause.Assignments(map[string]any{"name": m.Name, "slug": m.Slug, "description": m.Description, "seo_title": m.SEOTitle, "seo_description": m.SEODescription, "updated_at": time.Now()})}).Create(&m).Error
}

func (r *Repository) FindCategoryTranslation(ctx context.Context, categoryID uint, locale string) (*domainCMS.CategoryTranslation, error) {
	var m modelCMS.CategoryTranslation
	if err := r.conn(ctx).Where("category_id = ? AND locale = ?", categoryID, locale).First(&m).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &domainCMS.CategoryTranslation{CategoryID: m.CategoryID, Locale: m.Locale, Name: m.Name, Slug: m.Slug, Description: m.Description, SEOTitle: m.SEOTitle, SEODescription: m.SEODescription}, nil
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

func (r *Repository) UpdateCategory(ctx context.Context, id uint, enabled bool, sortOrder int) error {
	result := r.conn(ctx).Model(&modelCMS.Category{}).Where("id = ?", id).Updates(map[string]any{"is_enabled": enabled, "sort_order": sortOrder})
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
	return r.findArticle(ctx, id, false)
}
func (r *Repository) SetArticleCover(ctx context.Context, articleID uint, mediaID *uint) error {
	res := r.conn(ctx).Model(&modelCMS.Article{}).Where("id = ? AND deleted_at IS NULL", articleID).Update("cover_media_id", mediaID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}
func (r *Repository) FindArticleIncludingDeleted(ctx context.Context, id uint) (*domainCMS.Article, error) {
	return r.findArticle(ctx, id, true)
}
func (r *Repository) findArticle(ctx context.Context, id uint, includeDeleted bool) (*domainCMS.Article, error) {
	var m modelCMS.Article
	db := r.conn(ctx)
	if !includeDeleted {
		db = db.Where("deleted_at IS NULL")
	}
	if err := db.First(&m, id).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &domainCMS.Article{ID: m.ID, AuthorUserID: m.AuthorUserID, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, DeletedAt: m.DeletedAt}, nil
}

func (r *Repository) SoftDeleteArticle(ctx context.Context, id uint, deletedAt time.Time) error {
	result := r.conn(ctx).Model(&modelCMS.Article{}).Where("id = ? AND deleted_at IS NULL", id).Updates(map[string]any{"deleted_at": deletedAt})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}
func (r *Repository) RestoreArticle(ctx context.Context, id uint) error {
	result := r.conn(ctx).Model(&modelCMS.Article{}).Where("id = ? AND deleted_at IS NOT NULL", id).Update("deleted_at", nil)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (r *Repository) FindArticleTranslation(ctx context.Context, articleID uint, locale string) (*domainCMS.ArticleTranslation, error) {
	var m modelCMS.ArticleTranslation
	if err := r.conn(ctx).Where("article_id = ? AND locale = ?", articleID, locale).First(&m).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return translationEntity(m), nil
}

func (r *Repository) RedirectSourceExists(ctx context.Context, locale, sourcePath string) (bool, error) {
	var count int64
	err := r.conn(ctx).Model(&modelCMS.URLRedirect{}).Where("locale = ? AND source_path = ?", locale, sourcePath).Count(&count).Error
	return count > 0, err
}
func (r *Repository) SaveURLRedirect(ctx context.Context, redirect *domainCMS.URLRedirect) error {
	db := r.conn(ctx)
	if err := db.Model(&modelCMS.URLRedirect{}).Where("locale = ? AND target_path = ?", redirect.Locale, redirect.SourcePath).Update("target_path", redirect.TargetPath).Error; err != nil {
		return err
	}
	m := modelCMS.URLRedirect{Locale: redirect.Locale, SourcePath: redirect.SourcePath, TargetPath: redirect.TargetPath, StatusCode: redirect.StatusCode}
	if m.StatusCode == 0 {
		m.StatusCode = 301
	}
	return db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "locale"}, {Name: "source_path"}}, DoUpdates: clause.Assignments(map[string]any{"target_path": m.TargetPath, "status_code": m.StatusCode})}).Create(&m).Error
}
func (r *Repository) FindURLRedirect(ctx context.Context, locale, sourcePath string) (*domainCMS.URLRedirect, error) {
	var m modelCMS.URLRedirect
	if err := r.conn(ctx).Where("locale = ? AND source_path = ?", locale, sourcePath).First(&m).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &domainCMS.URLRedirect{Locale: m.Locale, SourcePath: m.SourcePath, TargetPath: m.TargetPath, StatusCode: m.StatusCode, CreatedAt: m.CreatedAt}, nil
}

func (r *Repository) ListArticleCategories(ctx context.Context, articleID uint) ([]domainCMS.ArticleCategory, error) {
	var models []modelCMS.ArticleCategory
	if err := r.conn(ctx).Where("article_id = ?", articleID).Order("is_primary DESC, category_id").Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]domainCMS.ArticleCategory, 0, len(models))
	for _, m := range models {
		result = append(result, domainCMS.ArticleCategory{CategoryID: m.CategoryID, IsPrimary: m.IsPrimary})
	}
	return result, nil
}
func (r *Repository) ListArticleTags(ctx context.Context, articleID uint, locale string) ([]*domainCMS.TagListItem, error) {
	type row struct {
		TagID      uint
		Name, Slug string
	}
	var rows []row
	err := r.conn(ctx).Table("article_tags").Joins("JOIN tags ON tags.id = article_tags.tag_id").Joins("JOIN tag_translations ON tag_translations.tag_id = tags.id").Where("article_tags.article_id = ? AND tag_translations.locale = ?", articleID, locale).Order("tag_translations.name, tags.id").Select("tags.id AS tag_id, tag_translations.name, tag_translations.slug").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make([]*domainCMS.TagListItem, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domainCMS.TagListItem{Tag: domainCMS.Tag{ID: row.TagID}, TagTranslation: domainCMS.TagTranslation{TagID: row.TagID, Locale: locale, Name: row.Name, Slug: row.Slug}})
	}
	return result, nil
}
func (r *Repository) ReplaceArticleTags(ctx context.Context, articleID uint, tagIDs []uint) error {
	db := r.conn(ctx)
	if err := db.Where("article_id = ?", articleID).Delete(&modelCMS.ArticleTag{}).Error; err != nil {
		return err
	}
	if len(tagIDs) == 0 {
		return nil
	}
	records := make([]modelCMS.ArticleTag, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		records = append(records, modelCMS.ArticleTag{ArticleID: articleID, TagID: tagID})
	}
	return db.Create(&records).Error
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

func (r *Repository) ListArticleTranslations(ctx context.Context, locale string, includeDeleted bool, page shared.PageQuery) ([]*domainCMS.ArticleListItem, int64, error) {
	db := r.conn(ctx).Table("article_translations").Joins("JOIN articles ON articles.id = article_translations.article_id").Where("article_translations.locale = ?", locale)
	if !includeDeleted {
		db = db.Where("articles.deleted_at IS NULL")
	}
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

func (r *Repository) ListPublishedArticleLocales(ctx context.Context, articleID uint) ([]domainCMS.PublishedLocale, error) {
	type row struct{ Locale, Slug string }
	var rows []row
	err := r.conn(ctx).Table("article_translations").Joins("JOIN locales ON locales.code = article_translations.locale").Where("article_translations.article_id = ? AND article_translations.status = 'published' AND article_translations.published_at <= NOW() AND locales.is_enabled", articleID).Order("locales.sort_order, article_translations.locale").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make([]domainCMS.PublishedLocale, 0, len(rows))
	for _, row := range rows {
		result = append(result, domainCMS.PublishedLocale{Locale: row.Locale, Slug: row.Slug})
	}
	return result, nil
}

func (r *Repository) ListPublicArticleBreadcrumbs(ctx context.Context, articleID uint, locale string) ([]domainCMS.CategoryTreeItem, error) {
	type row struct {
		CategoryID              uint
		ParentID                *uint
		SortOrder               int
		Name, Slug, Description string
	}
	var rows []row
	err := r.conn(ctx).Raw(`WITH RECURSIVE path AS (
  SELECT c.id, c.parent_id, c.sort_order, 1 AS depth
  FROM article_categories ac JOIN categories c ON c.id = ac.category_id
  WHERE ac.article_id = ? AND ac.is_primary AND c.is_enabled
  UNION ALL
  SELECT parent.id, parent.parent_id, parent.sort_order, path.depth + 1
  FROM categories parent JOIN path ON path.parent_id = parent.id
  WHERE parent.is_enabled
)
SELECT path.id AS category_id, path.parent_id, path.sort_order, ct.name, ct.slug, ct.description
FROM path JOIN category_translations ct ON ct.category_id = path.id AND ct.locale = ?
ORDER BY path.depth DESC`, articleID, locale).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make([]domainCMS.CategoryTreeItem, 0, len(rows))
	for _, row := range rows {
		result = append(result, domainCMS.CategoryTreeItem{Category: domainCMS.Category{ID: row.CategoryID, ParentID: row.ParentID, SortOrder: row.SortOrder, Enabled: true}, CategoryTranslation: domainCMS.CategoryTranslation{CategoryID: row.CategoryID, Locale: locale, Name: row.Name, Slug: row.Slug, Description: row.Description}})
	}
	return result, nil
}

func (r *Repository) ListPublicSitemapEntries(ctx context.Context, locale string, page shared.PageQuery) ([]domainCMS.SitemapEntry, int64, error) {
	base := `
SELECT 'article' AS kind, article_translations.slug, article_translations.updated_at
FROM article_translations
JOIN articles ON articles.id = article_translations.article_id
JOIN locales ON locales.code = article_translations.locale
WHERE article_translations.locale = ? AND article_translations.status = 'published' AND article_translations.published_at <= NOW() AND articles.deleted_at IS NULL AND locales.is_enabled
UNION ALL
SELECT 'category' AS kind, category_translations.slug, category_translations.updated_at
FROM category_translations
JOIN categories ON categories.id = category_translations.category_id
JOIN locales ON locales.code = category_translations.locale
WHERE category_translations.locale = ? AND categories.is_enabled AND locales.is_enabled`
	var total int64
	if err := r.conn(ctx).Raw("SELECT COUNT(*) FROM ("+base+") AS sitemap_entries", locale, locale).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	type row struct {
		Kind, Slug string
		UpdatedAt  time.Time
	}
	var rows []row
	query := "SELECT kind, slug, updated_at FROM (" + base + ") AS sitemap_entries ORDER BY updated_at DESC, kind, slug LIMIT ? OFFSET ?"
	if err := r.conn(ctx).Raw(query, locale, locale, page.PerPage, page.Offset()).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	result := make([]domainCMS.SitemapEntry, 0, len(rows))
	for _, row := range rows {
		result = append(result, domainCMS.SitemapEntry{Kind: row.Kind, Slug: row.Slug, UpdatedAt: row.UpdatedAt})
	}
	return result, total, nil
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

func (r *Repository) PublicTagExists(ctx context.Context, locale, slug string) (bool, error) {
	var count int64
	err := r.conn(ctx).Table("tag_translations").Joins("JOIN locales ON locales.code = tag_translations.locale").Where("tag_translations.locale = ? AND tag_translations.slug = ? AND locales.is_enabled", locale, slug).Count(&count).Error
	return count == 1, err
}
func (r *Repository) ListPublicTagArticles(ctx context.Context, locale, tagSlug string, page shared.PageQuery) ([]*domainCMS.PublicArticleListItem, int64, error) {
	db := r.conn(ctx).Table("article_translations").Joins("JOIN articles ON articles.id = article_translations.article_id").Joins("LEFT JOIN article_categories primary_ac ON primary_ac.article_id = articles.id AND primary_ac.is_primary").Joins("LEFT JOIN categories primary_c ON primary_c.id = primary_ac.category_id AND primary_c.is_enabled").Joins("LEFT JOIN category_translations primary_ct ON primary_ct.category_id = primary_c.id AND primary_ct.locale = article_translations.locale").Joins("JOIN article_tags filter_at ON filter_at.article_id = articles.id").Joins("JOIN tag_translations filter_tt ON filter_tt.tag_id = filter_at.tag_id AND filter_tt.locale = article_translations.locale").Where("articles.deleted_at IS NULL AND article_translations.locale = ? AND article_translations.status = 'published' AND article_translations.published_at <= NOW() AND filter_tt.slug = ?", locale, tagSlug)
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
	if err := db.Select("articles.id AS article_id, article_translations.title, article_translations.slug, article_translations.summary, article_translations.content_format, article_translations.published_at, article_translations.updated_at, primary_c.id AS primary_category_id, primary_ct.name AS primary_category_name, primary_ct.slug AS primary_category_slug").Order("article_translations.published_at DESC, article_translations.id DESC").Limit(page.PerPage).Offset(page.Offset()).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	result := make([]*domainCMS.PublicArticleListItem, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domainCMS.PublicArticleListItem{Article: domainCMS.Article{ID: row.ArticleID, UpdatedAt: row.UpdatedAt}, ArticleTranslation: domainCMS.ArticleTranslation{ArticleID: row.ArticleID, Locale: locale, Title: row.Title, Slug: row.Slug, Summary: row.Summary, ContentFormat: row.ContentFormat, PublishedAt: row.PublishedAt, Status: domainCMS.TranslationPublished, UpdatedAt: row.UpdatedAt}, PrimaryCategoryID: row.PrimaryCategoryID, PrimaryCategoryName: row.PrimaryCategoryName, PrimaryCategorySlug: row.PrimaryCategorySlug})
	}
	return result, total, nil
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
func localeModel(locale *domainCMS.Locale) modelCMS.Locale {
	return modelCMS.Locale{Code: locale.Code, Name: locale.Name, IsDefault: locale.IsDefault, IsEnabled: locale.IsEnabled, SortOrder: locale.SortOrder}
}
func localeEntity(m modelCMS.Locale) *domainCMS.Locale {
	return &domainCMS.Locale{Code: m.Code, Name: m.Name, IsDefault: m.IsDefault, IsEnabled: m.IsEnabled, SortOrder: m.SortOrder, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
}
