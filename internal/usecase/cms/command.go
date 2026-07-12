package cms

import "github.com/freeDog-wy/go-backend-template/internal/domain/shared"

type CreateCategoryCmd struct {
	ParentID                                                  *uint
	SortOrder                                                 int
	Locale, Name, Slug, Description, SEOTitle, SEODescription string
}
type MoveCategoryCmd struct {
	CategoryID    uint
	ParentID      *uint
	SortOrder     int
	ActorUserID   uint
	IP, UserAgent string
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
	ActorUserID                                                                                  uint
	IP, UserAgent                                                                                string
}
type PublishTranslationCmd struct {
	ArticleID     uint
	Locale        string
	ActorUserID   uint
	IP, UserAgent string
}
type ArchiveTranslationCmd struct {
	ArticleID     uint
	Locale        string
	ActorUserID   uint
	IP, UserAgent string
}
type ListCategoriesCmd struct{ Locale string }
type ReplaceArticleCategoriesCmd struct {
	ArticleID         uint
	CategoryIDs       []uint
	PrimaryCategoryID *uint
}
type ListArticlesCmd struct {
	Locale         string
	IncludeDeleted bool
	Page           shared.PageQuery
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
type ListPublicSitemapEntriesCmd struct {
	Locale string
	Page   shared.PageQuery
}
type CreateLocaleCmd struct {
	Code, Name    string
	SortOrder     int
	ActorUserID   uint
	IP, UserAgent string
}
type UpdateLocaleCmd struct {
	Code          string
	Name          string
	IsEnabled     bool
	SortOrder     int
	IsDefault     bool
	ActorUserID   uint
	IP, UserAgent string
}
type UpsertCategoryTranslationCmd struct {
	CategoryID                                                uint
	Locale, Name, Slug, Description, SEOTitle, SEODescription string
	ActorUserID                                               uint
	IP, UserAgent                                             string
}
type GetArticleTranslationCmd struct {
	ArticleID uint
	Locale    string
}
type UpdateCategoryCmd struct {
	CategoryID    uint
	IsEnabled     bool
	SortOrder     int
	ActorUserID   uint
	IP, UserAgent string
}
type DeleteArticleCmd struct {
	ArticleID     uint
	ActorUserID   uint
	IP, UserAgent string
}
type RestoreArticleCmd struct {
	ArticleID     uint
	ActorUserID   uint
	IP, UserAgent string
}
type SetArticleCoverCmd struct { ArticleID uint; MediaID *uint; ActorUserID uint; IP, UserAgent string }
type CreateTagCmd struct {
	Locale, Name, Slug string
	ActorUserID        uint
	IP, UserAgent      string
}
type UpsertTagTranslationCmd struct {
	TagID              uint
	Locale, Name, Slug string
	ActorUserID        uint
	IP, UserAgent      string
}
type ListTagsCmd struct {
	Locale string
	Page   shared.PageQuery
}
type ReplaceArticleTagsCmd struct {
	ArticleID     uint
	TagIDs        []uint
	ActorUserID   uint
	IP, UserAgent string
}
type ListPublicTagArticlesCmd struct {
	Locale, TagSlug string
	Page            shared.PageQuery
}
