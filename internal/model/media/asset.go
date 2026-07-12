package media

import "time"

type Asset struct {
	ID                                            uint `gorm:"primaryKey"`
	UploaderUserID                                uint
	ObjectKey, OriginalFilename, MimeType, Status string
	SizeBytes                                     int64
	Width, Height                                 int
	CreatedAt, UpdatedAt                          time.Time
	DeletedAt                                     *time.Time
}

func (Asset) TableName() string { return "media_assets" }

type Translation struct {
	ID                     uint `gorm:"primaryKey"`
	MediaID                uint
	Locale, AltText, Title string
	CreatedAt, UpdatedAt   time.Time
}

func (Translation) TableName() string { return "media_translations" }
