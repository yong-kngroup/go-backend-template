package cms

import (
	"context"

	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
)

type AdminService interface {
	CreateCategory(context.Context, CreateCategoryCmd) (*CategoryResult, error)
	MoveCategory(context.Context, MoveCategoryCmd) error
	CreateArticle(context.Context, CreateArticleCmd) (*ArticleResult, error)
	CreateTranslation(context.Context, CreateTranslationCmd) (*ArticleResult, error)
	UpdateTranslation(context.Context, UpdateTranslationCmd) (*ArticleResult, error)
	PublishTranslation(context.Context, PublishTranslationCmd) (*ArticleResult, error)
	ArchiveTranslation(context.Context, ArchiveTranslationCmd) (*ArticleResult, error)
	ListCategories(context.Context, ListCategoriesCmd) ([]*CategoryTreeResult, error)
	ReplaceArticleCategories(context.Context, ReplaceArticleCategoriesCmd) error
	ListArticles(context.Context, ListArticlesCmd) ([]*ArticleResult, shared.PageResult, error)
}

type PublicContentService interface {
	GetPublishedArticle(context.Context, string, string) (*PublicArticleResult, error)
	ListPublishedArticles(context.Context, ListPublicArticlesCmd) ([]*PublicArticleListResult, shared.PageResult, error)
	ListPublishedCategoryArticles(context.Context, ListPublicCategoryArticlesCmd) ([]*PublicArticleListResult, shared.PageResult, error)
	ListPublishedCategories(context.Context, string) ([]*CategoryTreeResult, error)
}

var _ AdminService = (*Service)(nil)
var _ PublicContentService = (*Service)(nil)
