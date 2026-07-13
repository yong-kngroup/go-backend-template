package mcp

import (
	"time"

	domainMCP "github.com/freeDog-wy/go-backend-template/internal/domain/mcp"
)

type ServiceAccount struct {
	ID                       uint   `gorm:"primaryKey"`
	UserID                   uint   `gorm:"not null;uniqueIndex"`
	ClientID                 string `gorm:"type:varchar(191);not null;uniqueIndex"`
	ClientSecretHash         string `gorm:"type:varchar(255);not null"`
	PreviousClientSecretHash string
	PreviousSecretExpiresAt  *time.Time
	Enabled                  bool `gorm:"not null"`
	DisabledAt               *time.Time
	CreatedAt                time.Time `gorm:"not null"`
	UpdatedAt                time.Time `gorm:"not null"`
}

func (a *ServiceAccount) ToEntity() *domainMCP.ServiceAccount {
	return domainMCP.ReconstituteServiceAccount(a.ID, a.UserID, a.ClientID, a.ClientSecretHash, a.PreviousClientSecretHash, a.PreviousSecretExpiresAt, a.Enabled, a.DisabledAt, a.CreatedAt, a.UpdatedAt)
}

func FromEntity(a *domainMCP.ServiceAccount) *ServiceAccount {
	return &ServiceAccount{ID: a.GetID(), UserID: a.GetUserID(), ClientID: a.GetClientID(), ClientSecretHash: a.GetClientSecretHash(), PreviousClientSecretHash: a.GetPreviousClientSecretHash(), PreviousSecretExpiresAt: a.GetPreviousSecretExpiresAt(), Enabled: a.IsEnabled(), DisabledAt: a.GetDisabledAt()}
}
