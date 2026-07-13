package auth

import "time"

type AccessClaims struct {
	UserID    uint
	SessionID string
	Type      string
	Issuer    string
	Audience  string
	ActorType string
	TokenID   string
	IssuedAt  time.Time
	ExpiresAt time.Time
}
