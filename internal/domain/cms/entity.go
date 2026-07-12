package cms

import "time"

type Category struct {
	ID        uint
	ParentID  *uint
	SortOrder int
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CategoryTranslation struct {
	CategoryID     uint
	Locale         string
	Name           string
	Slug           string
	Description    string
	SEOTitle       string
	SEODescription string
}

type Article struct {
	ID           uint
	AuthorUserID uint
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}

type TranslationStatus string

const (
	TranslationDraft     TranslationStatus = "draft"
	TranslationPublished TranslationStatus = "published"
	TranslationArchived  TranslationStatus = "archived"
)

type ArticleTranslation struct {
	ID             uint
	ArticleID      uint
	Locale         string
	Title          string
	Slug           string
	Summary        string
	Content        string
	ContentFormat  string
	Status         TranslationStatus
	PublishedAt    *time.Time
	SEOTitle       string
	SEODescription string
	CanonicalURL   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type PublicArticle struct {
	Article
	ArticleTranslation
}

type CategoryTreeItem struct {
	Category
	CategoryTranslation
}

type ArticleListItem struct {
	Article
	ArticleTranslation
}

type PublicArticleListItem struct {
	Article
	ArticleTranslation
	PrimaryCategoryID   *uint
	PrimaryCategoryName string
	PrimaryCategorySlug string
}
