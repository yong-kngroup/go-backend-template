package cms

import "time"

type LocaleResult struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
	IsEnabled bool   `json:"is_enabled"`
	SortOrder int    `json:"sort_order"`
}

type CategoryResult struct {
	ID        uint   `json:"id"`
	ParentID  *uint  `json:"parent_id,omitempty"`
	SortOrder int    `json:"sort_order"`
	Locale    string `json:"locale"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
}
type CategoryTreeResult struct {
	ID          uint                  `json:"id"`
	ParentID    *uint                 `json:"parent_id,omitempty"`
	SortOrder   int                   `json:"sort_order"`
	Name        string                `json:"name"`
	Slug        string                `json:"slug"`
	Description string                `json:"description"`
	Children    []*CategoryTreeResult `json:"children"`
}
type ArticleResult struct {
	ID          uint       `json:"id"`
	Locale      string     `json:"locale"`
	Title       string     `json:"title"`
	Slug        string     `json:"slug"`
	Status      string     `json:"status"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}
type ArticleDetailResult struct {
	ID             uint                    `json:"id"`
	AuthorUserID   uint                    `json:"author_user_id"`
	Locale         string                  `json:"locale"`
	Title          string                  `json:"title"`
	Slug           string                  `json:"slug"`
	Summary        string                  `json:"summary"`
	Content        string                  `json:"content"`
	ContentFormat  string                  `json:"content_format"`
	Status         string                  `json:"status"`
	PublishedAt    *time.Time              `json:"published_at,omitempty"`
	SEOTitle       string                  `json:"seo_title"`
	SEODescription string                  `json:"seo_description"`
	CanonicalURL   string                  `json:"canonical_url"`
	Categories     []ArticleCategoryResult `json:"categories"`
	Tags           []TagResult             `json:"tags"`
}
type TagResult struct {
	ID     uint   `json:"id"`
	Locale string `json:"locale"`
	Name   string `json:"name"`
	Slug   string `json:"slug"`
}
type ArticleCategoryResult struct {
	CategoryID uint `json:"category_id"`
	IsPrimary  bool `json:"is_primary"`
}
type PublicArticleResult struct {
	ID               uint                `json:"id"`
	Locale           string              `json:"locale"`
	Title            string              `json:"title"`
	Slug             string              `json:"slug"`
	Summary          string              `json:"summary"`
	Content          string              `json:"content"`
	ContentFormat    string              `json:"content_format"`
	PublishedAt      *time.Time          `json:"published_at"`
	SEOTitle         string              `json:"seo_title"`
	SEODescription   string              `json:"seo_description"`
	CanonicalURL     string              `json:"canonical_url"`
	UpdatedAt        time.Time           `json:"updated_at"`
	AvailableLocales []PublicLocaleRef   `json:"available_locales"`
	PrimaryCategory  *PublicCategoryRef  `json:"primary_category,omitempty"`
	Breadcrumbs      []PublicCategoryRef `json:"breadcrumbs"`
}
type PublicLocaleRef struct {
	Locale string `json:"locale"`
	Slug   string `json:"slug"`
}
type PublicArticleListResult struct {
	ID              uint               `json:"id"`
	Locale          string             `json:"locale"`
	Title           string             `json:"title"`
	Slug            string             `json:"slug"`
	Summary         string             `json:"summary"`
	ContentFormat   string             `json:"content_format"`
	PublishedAt     *time.Time         `json:"published_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
	PrimaryCategory *PublicCategoryRef `json:"primary_category,omitempty"`
}
type PublicCategoryRef struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}
type SitemapEntryResult struct {
	URL          string    `json:"url"`
	LastModified time.Time `json:"last_modified"`
}
type RedirectResult struct {
	SourcePath string `json:"source_path"`
	TargetPath string `json:"target_path"`
	StatusCode int    `json:"status_code"`
}
