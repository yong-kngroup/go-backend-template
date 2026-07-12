package cms

import (
	"context"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
)

type Repository interface {
	LocaleEnabled(ctx context.Context, code string) (bool, error)
	ListLocales(ctx context.Context) ([]*Locale, error)
	FindLocale(ctx context.Context, code string) (*Locale, error)
	CreateLocale(ctx context.Context, locale *Locale) error
	UpdateLocale(ctx context.Context, locale *Locale) error
	SetDefaultLocale(ctx context.Context, code string) error
	CountEnabledLocales(ctx context.Context) (int64, error)
	CreateTag(ctx context.Context, tag *Tag, translation *TagTranslation) error
	FindTag(ctx context.Context, id uint) (*Tag, error)
	FindTagTranslation(ctx context.Context, tagID uint, locale string) (*TagTranslation, error)
	UpsertTagTranslation(ctx context.Context, translation *TagTranslation) error
	ListTags(ctx context.Context, locale string, page shared.PageQuery) ([]*TagListItem, int64, error)
	CreateCategory(ctx context.Context, category *Category, translation *CategoryTranslation) error
	UpsertCategoryTranslation(ctx context.Context, translation *CategoryTranslation) error
	FindCategoryTranslation(ctx context.Context, categoryID uint, locale string) (*CategoryTranslation, error)
	FindCategory(ctx context.Context, id uint) (*Category, error)
	IsCategoryDescendant(ctx context.Context, ancestorID, candidateID uint) (bool, error)
	MoveCategory(ctx context.Context, id uint, parentID *uint, sortOrder int) error
	UpdateCategory(ctx context.Context, id uint, enabled bool, sortOrder int) error
	ListCategories(ctx context.Context) ([]*Category, error)
	ListCategoryTreeItems(ctx context.Context, locale string) ([]*CategoryTreeItem, error)
	CreateArticle(ctx context.Context, article *Article, translation *ArticleTranslation) error
	FindArticle(ctx context.Context, id uint) (*Article, error)
	FindArticleIncludingDeleted(ctx context.Context, id uint) (*Article, error)
	SoftDeleteArticle(ctx context.Context, id uint, deletedAt time.Time) error
	RestoreArticle(ctx context.Context, id uint) error
	CreateArticleTranslation(ctx context.Context, translation *ArticleTranslation) error
	FindArticleTranslation(ctx context.Context, articleID uint, locale string) (*ArticleTranslation, error)
	RedirectSourceExists(ctx context.Context, locale, sourcePath string) (bool, error)
	SaveURLRedirect(ctx context.Context, redirect *URLRedirect) error
	FindURLRedirect(ctx context.Context, locale, sourcePath string) (*URLRedirect, error)
	ListArticleCategories(ctx context.Context, articleID uint) ([]ArticleCategory, error)
	ListArticleTags(ctx context.Context, articleID uint, locale string) ([]*TagListItem, error)
	ReplaceArticleTags(ctx context.Context, articleID uint, tagIDs []uint) error
	SaveArticleTranslation(ctx context.Context, translation *ArticleTranslation) error
	ReplaceArticleCategories(ctx context.Context, articleID uint, categoryIDs []uint, primaryCategoryID *uint) error
	ListArticleTranslations(ctx context.Context, locale string, includeDeleted bool, page shared.PageQuery) ([]*ArticleListItem, int64, error)
	FindPublicArticle(ctx context.Context, locale, slug string) (*PublicArticle, error)
	ListPublishedArticleLocales(ctx context.Context, articleID uint) ([]PublishedLocale, error)
	ListPublicArticleBreadcrumbs(ctx context.Context, articleID uint, locale string) ([]CategoryTreeItem, error)
	ListPublicSitemapEntries(ctx context.Context, locale string, page shared.PageQuery) ([]SitemapEntry, int64, error)
	ListPublicCategoryTreeItems(ctx context.Context, locale string) ([]*CategoryTreeItem, error)
	PublicCategoryExists(ctx context.Context, locale, slug string) (bool, error)
	ListPublicArticles(ctx context.Context, locale string, categorySlug *string, page shared.PageQuery) ([]*PublicArticleListItem, int64, error)
	PublicTagExists(ctx context.Context, locale, slug string) (bool, error)
	ListPublicTagArticles(ctx context.Context, locale, tagSlug string, page shared.PageQuery) ([]*PublicArticleListItem, int64, error)
}
