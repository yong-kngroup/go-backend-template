package cms

import (
	"context"

	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
)

type AdminService interface {
	CreateTag(context.Context, CreateTagCmd) (*TagResult, error)
	UpsertTagTranslation(context.Context, UpsertTagTranslationCmd) (*TagResult, error)
	ListTags(context.Context, ListTagsCmd) ([]*TagResult, shared.PageResult, error)
	ListLocales(context.Context) ([]*LocaleResult, error)
	CreateLocale(context.Context, CreateLocaleCmd) (*LocaleResult, error)
	UpdateLocale(context.Context, UpdateLocaleCmd) (*LocaleResult, error)
	CreateCategory(context.Context, CreateCategoryCmd) (*CategoryResult, error)
	UpsertCategoryTranslation(context.Context, UpsertCategoryTranslationCmd) (*CategoryResult, error)
	MoveCategory(context.Context, MoveCategoryCmd) error
	UpdateCategory(context.Context, UpdateCategoryCmd) (*CategoryResult, error)
	CreateArticle(context.Context, CreateArticleCmd) (*ArticleResult, error)
	CreateTranslation(context.Context, CreateTranslationCmd) (*ArticleResult, error)
	UpdateTranslation(context.Context, UpdateTranslationCmd) (*ArticleResult, error)
	PublishTranslation(context.Context, PublishTranslationCmd) (*ArticleResult, error)
	PreviewPublish(context.Context, PreviewPublishCmd) (*PublishPreviewResult, error)
	ArchiveTranslation(context.Context, ArchiveTranslationCmd) (*ArticleResult, error)
	DeleteArticle(context.Context, DeleteArticleCmd) error
	RestoreArticle(context.Context, RestoreArticleCmd) error
	SetArticleCover(context.Context, SetArticleCoverCmd) error
	ListCategories(context.Context, ListCategoriesCmd) ([]*CategoryTreeResult, error)
	ReplaceArticleCategories(context.Context, ReplaceArticleCategoriesCmd) error
	ReplaceArticleTags(context.Context, ReplaceArticleTagsCmd) error
	ListArticles(context.Context, ListArticlesCmd) ([]*ArticleResult, shared.PageResult, error)
	GetArticleTranslation(context.Context, GetArticleTranslationCmd) (*ArticleDetailResult, error)
}

type PublicContentService interface {
	ListPublishedLocales(context.Context) ([]*LocaleResult, error)
	GetPublishedArticle(context.Context, string, string) (*PublicArticleResult, error)
	ListPublishedArticles(context.Context, ListPublicArticlesCmd) ([]*PublicArticleListResult, shared.PageResult, error)
	ListPublishedCategoryArticles(context.Context, ListPublicCategoryArticlesCmd) ([]*PublicArticleListResult, shared.PageResult, error)
	ListPublishedCategories(context.Context, string) ([]*CategoryTreeResult, error)
	ListPublicSitemapEntries(context.Context, ListPublicSitemapEntriesCmd) ([]*SitemapEntryResult, shared.PageResult, error)
	ResolveRedirect(context.Context, string, string) (*RedirectResult, error)
	ListPublicRedirects(context.Context, ListPublicRedirectsCmd) ([]*RedirectResult, shared.PageResult, error)
	ListPublishedTags(context.Context, ListPublicTagsCmd) ([]*TagResult, shared.PageResult, error)
	ListPublishedTagArticles(context.Context, ListPublicTagArticlesCmd) ([]*PublicArticleListResult, shared.PageResult, error)
}

var _ AdminService = (*Service)(nil)
var _ PublicContentService = (*Service)(nil)
