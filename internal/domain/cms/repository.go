package cms

import (
	"context"

	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
)

type Repository interface {
	LocaleEnabled(ctx context.Context, code string) (bool, error)
	CreateCategory(ctx context.Context, category *Category, translation *CategoryTranslation) error
	FindCategory(ctx context.Context, id uint) (*Category, error)
	IsCategoryDescendant(ctx context.Context, ancestorID, candidateID uint) (bool, error)
	MoveCategory(ctx context.Context, id uint, parentID *uint, sortOrder int) error
	ListCategories(ctx context.Context) ([]*Category, error)
	ListCategoryTreeItems(ctx context.Context, locale string) ([]*CategoryTreeItem, error)
	CreateArticle(ctx context.Context, article *Article, translation *ArticleTranslation) error
	FindArticle(ctx context.Context, id uint) (*Article, error)
	CreateArticleTranslation(ctx context.Context, translation *ArticleTranslation) error
	FindArticleTranslation(ctx context.Context, articleID uint, locale string) (*ArticleTranslation, error)
	SaveArticleTranslation(ctx context.Context, translation *ArticleTranslation) error
	ReplaceArticleCategories(ctx context.Context, articleID uint, categoryIDs []uint, primaryCategoryID *uint) error
	ListArticleTranslations(ctx context.Context, locale string, page shared.PageQuery) ([]*ArticleListItem, int64, error)
	FindPublicArticle(ctx context.Context, locale, slug string) (*PublicArticle, error)
}
