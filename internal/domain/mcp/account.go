package mcp

import (
	"strings"
	"time"
)

type ServiceAccount struct {
	id                       uint
	userID                   uint
	clientID                 string
	clientSecretHash         string
	previousClientSecretHash string
	previousSecretExpiresAt  *time.Time
	enabled                  bool
	disabledAt               *time.Time
	createdAt                time.Time
	updatedAt                time.Time
}

func NewServiceAccount(userID uint, clientID, clientSecretHash string, now time.Time) (*ServiceAccount, error) {
	clientID = strings.TrimSpace(clientID)
	if userID == 0 || clientID == "" || strings.TrimSpace(clientSecretHash) == "" || now.IsZero() {
		return nil, ErrInvalidServiceAccount
	}
	return &ServiceAccount{
		userID:           userID,
		clientID:         clientID,
		clientSecretHash: clientSecretHash,
		enabled:          true,
		createdAt:        now,
		updatedAt:        now,
	}, nil
}

func ReconstituteServiceAccount(id, userID uint, clientID, clientSecretHash, previousClientSecretHash string, previousSecretExpiresAt *time.Time, enabled bool, disabledAt *time.Time, createdAt, updatedAt time.Time) *ServiceAccount {
	return &ServiceAccount{id: id, userID: userID, clientID: clientID, clientSecretHash: clientSecretHash, previousClientSecretHash: previousClientSecretHash, previousSecretExpiresAt: previousSecretExpiresAt, enabled: enabled, disabledAt: disabledAt, createdAt: createdAt, updatedAt: updatedAt}
}

func (a *ServiceAccount) RotateSecret(hash string, graceUntil, now time.Time) error {
	if strings.TrimSpace(hash) == "" || !graceUntil.After(now) || now.IsZero() {
		return ErrInvalidServiceAccount
	}
	a.previousClientSecretHash = a.clientSecretHash
	a.previousSecretExpiresAt = &graceUntil
	a.clientSecretHash = hash
	a.updatedAt = now
	return nil
}

func (a *ServiceAccount) Disable(now time.Time) error {
	if now.IsZero() {
		return ErrInvalidServiceAccount
	}
	a.enabled = false
	a.disabledAt = &now
	a.updatedAt = now
	return nil
}

func (a *ServiceAccount) Enable(now time.Time) error {
	if now.IsZero() {
		return ErrInvalidServiceAccount
	}
	a.enabled = true
	a.disabledAt = nil
	a.updatedAt = now
	return nil
}

func (a *ServiceAccount) AssignID(id uint) {
	if a.id == 0 {
		a.id = id
	}
}
func (a *ServiceAccount) GetID() uint                            { return a.id }
func (a *ServiceAccount) GetUserID() uint                        { return a.userID }
func (a *ServiceAccount) GetClientID() string                    { return a.clientID }
func (a *ServiceAccount) GetClientSecretHash() string            { return a.clientSecretHash }
func (a *ServiceAccount) GetPreviousClientSecretHash() string    { return a.previousClientSecretHash }
func (a *ServiceAccount) GetPreviousSecretExpiresAt() *time.Time { return a.previousSecretExpiresAt }
func (a *ServiceAccount) GetDisabledAt() *time.Time              { return a.disabledAt }
func (a *ServiceAccount) PreviousSecretActive(now time.Time) bool {
	return a.previousClientSecretHash != "" && a.previousSecretExpiresAt != nil && a.previousSecretExpiresAt.After(now)
}
func (a *ServiceAccount) IsEnabled() bool { return a.enabled }
