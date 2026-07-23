package sitegen

import "time"

type Locale struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
	IsEnabled bool   `json:"is_enabled"`
	SortOrder int    `json:"sort_order"`
}

type Category struct {
	ID          uint       `json:"id"`
	ParentID    *uint      `json:"parent_id,omitempty"`
	SortOrder   int        `json:"sort_order"`
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Description string     `json:"description"`
	Children    []Category `json:"children"`
}

type Tag struct {
	ID     uint   `json:"id"`
	Locale string `json:"locale"`
	Name   string `json:"name"`
	Slug   string `json:"slug"`
}

type Cover struct {
	ID      uint   `json:"id"`
	URL     string `json:"url"`
	AltText string `json:"alt_text"`
	Title   string `json:"title"`
}

type CategoryRef struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type ArticleListItem struct {
	ID              uint         `json:"id"`
	Locale          string       `json:"locale"`
	Title           string       `json:"title"`
	Slug            string       `json:"slug"`
	Summary         string       `json:"summary"`
	ContentFormat   string       `json:"content_format"`
	PublishedAt     *time.Time   `json:"published_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
	Cover           *Cover       `json:"cover,omitempty"`
	PrimaryCategory *CategoryRef `json:"primary_category,omitempty"`
}

type ArticleLocale struct {
	Locale string `json:"locale"`
	Slug   string `json:"slug"`
}

type Article struct {
	ID               uint            `json:"id"`
	Locale           string          `json:"locale"`
	Title            string          `json:"title"`
	Slug             string          `json:"slug"`
	Summary          string          `json:"summary"`
	Content          string          `json:"content"`
	ContentFormat    string          `json:"content_format"`
	PublishedAt      *time.Time      `json:"published_at"`
	SEOTitle         string          `json:"seo_title"`
	SEODescription   string          `json:"seo_description"`
	CanonicalURL     string          `json:"canonical_url"`
	Cover            *Cover          `json:"cover,omitempty"`
	UpdatedAt        time.Time       `json:"updated_at"`
	AvailableLocales []ArticleLocale `json:"available_locales"`
	PrimaryCategory  *CategoryRef    `json:"primary_category,omitempty"`
	Breadcrumbs      []CategoryRef   `json:"breadcrumbs"`
}

type SitemapEntry struct {
	URL          string    `json:"url"`
	LastModified time.Time `json:"last_modified"`
}

type Redirect struct {
	SourcePath string `json:"source_path"`
	TargetPath string `json:"target_path"`
	StatusCode int    `json:"status_code"`
}

type pageMeta struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type apiResponse[T any] struct {
	Success bool      `json:"success"`
	Data    T         `json:"data"`
	Error   *apiError `json:"error"`
	Meta    *pageMeta `json:"meta"`
}
