package cms

import "time"

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
type PublicArticleResult struct {
	ID             uint       `json:"id"`
	Locale         string     `json:"locale"`
	Title          string     `json:"title"`
	Slug           string     `json:"slug"`
	Summary        string     `json:"summary"`
	Content        string     `json:"content"`
	ContentFormat  string     `json:"content_format"`
	PublishedAt    *time.Time `json:"published_at"`
	SEOTitle       string     `json:"seo_title"`
	SEODescription string     `json:"seo_description"`
	CanonicalURL   string     `json:"canonical_url"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
