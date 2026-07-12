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
