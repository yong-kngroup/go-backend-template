package cms

import "github.com/freeDog-wy/go-backend-template/internal/domain/shared"

type CreateCategoryCmd struct {
	ParentID                                                  *uint
	SortOrder                                                 int
	Locale, Name, Slug, Description, SEOTitle, SEODescription string
}
type MoveCategoryCmd struct {
	CategoryID uint
	ParentID   *uint
	SortOrder  int
}
type CreateArticleCmd struct {
	AuthorUserID                                                                                 uint
	Locale, Title, Slug, Summary, Content, ContentFormat, SEOTitle, SEODescription, CanonicalURL string
}
type CreateTranslationCmd struct {
	ArticleID                                                                                    uint
	Locale, Title, Slug, Summary, Content, ContentFormat, SEOTitle, SEODescription, CanonicalURL string
}
type UpdateTranslationCmd struct {
	ArticleID                                                                                    uint
	Locale, Title, Slug, Summary, Content, ContentFormat, SEOTitle, SEODescription, CanonicalURL string
}
type PublishTranslationCmd struct {
	ArticleID uint
	Locale    string
}
type ArchiveTranslationCmd struct {
	ArticleID uint
	Locale    string
}
type ListCategoriesCmd struct{ Locale string }
type ReplaceArticleCategoriesCmd struct {
	ArticleID         uint
	CategoryIDs       []uint
	PrimaryCategoryID *uint
}
type ListArticlesCmd struct {
	Locale string
	Page   shared.PageQuery
}
type ListPublicArticlesCmd struct {
	Locale string
	Page   shared.PageQuery
}
type ListPublicCategoryArticlesCmd struct {
	Locale       string
	CategorySlug string
	Page         shared.PageQuery
}
