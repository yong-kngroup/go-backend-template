package cms

import "time"

type Locale struct {
	Code      string `gorm:"primaryKey"`
	Name      string
	IsDefault bool
	IsEnabled bool
	SortOrder int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Locale) TableName() string { return "locales" }

type Category struct {
	ID        uint `gorm:"primaryKey"`
	ParentID  *uint
	SortOrder int
	IsEnabled bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Category) TableName() string { return "categories" }

type CategoryTranslation struct {
	ID             uint `gorm:"primaryKey"`
	CategoryID     uint
	Locale         string
	Name           string
	Slug           string
	Description    string
	SEOTitle       string `gorm:"column:seo_title"`
	SEODescription string `gorm:"column:seo_description"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (CategoryTranslation) TableName() string { return "category_translations" }

type Article struct {
	ID           uint `gorm:"primaryKey"`
	AuthorUserID uint
	CoverMediaID *uint
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}

func (Article) TableName() string { return "articles" }

type ArticleTranslation struct {
	ID             uint `gorm:"primaryKey"`
	ArticleID      uint
	Locale         string
	Title          string
	Slug           string
	Summary        string
	Content        string
	ContentFormat  string
	Status         string
	PublishedAt    *time.Time
	SEOTitle       string `gorm:"column:seo_title"`
	SEODescription string `gorm:"column:seo_description"`
	CanonicalURL   string `gorm:"column:canonical_url"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (ArticleTranslation) TableName() string { return "article_translations" }

type ArticleCategory struct {
	ArticleID  uint `gorm:"primaryKey"`
	CategoryID uint `gorm:"primaryKey"`
	IsPrimary  bool
}

func (ArticleCategory) TableName() string { return "article_categories" }

type URLRedirect struct {
	ID         uint `gorm:"primaryKey"`
	Locale     string
	SourcePath string
	TargetPath string
	StatusCode int
	CreatedAt  time.Time
}

func (URLRedirect) TableName() string { return "url_redirects" }

type Tag struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Tag) TableName() string { return "tags" }

type TagTranslation struct {
	ID        uint `gorm:"primaryKey"`
	TagID     uint
	Locale    string
	Name      string
	Slug      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (TagTranslation) TableName() string { return "tag_translations" }

type ArticleTag struct {
	ArticleID uint `gorm:"primaryKey"`
	TagID     uint `gorm:"primaryKey"`
}

func (ArticleTag) TableName() string { return "article_tags" }
