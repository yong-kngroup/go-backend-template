package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	domainAuth "github.com/freeDog-wy/go-backend-template/internal/domain/auth"
)

type JWTManager struct {
	issuer              string
	audience            string
	additionalAudiences map[string]struct{}
	secret              []byte
}

type jwtClaims struct {
	Sub       string `json:"sub"`
	SID       string `json:"sid"`
	Typ       string `json:"typ"`
	Iss       string `json:"iss"`
	Aud       string `json:"aud"`
	ActorType string `json:"actor_type,omitempty"`
	JTI       string `json:"jti,omitempty"`
	Iat       int64  `json:"iat"`
	Exp       int64  `json:"exp"`
}

func NewJWTManager(issuer, audience, secret string) (*JWTManager, error) {
	secret = strings.TrimSpace(secret)
	if len(secret) < 32 {
		return nil, fmt.Errorf("jwt secret must contain at least 32 bytes")
	}
	if secret == "change-me" || strings.Contains(strings.ToLower(secret), "replace-with") {
		return nil, fmt.Errorf("jwt secret must not use an example value")
	}
	return &JWTManager{issuer: issuer, audience: audience, additionalAudiences: make(map[string]struct{}), secret: []byte(secret)}, nil
}

// AllowAudience permits a separately scoped access-token audience to be
// authenticated by the same CMS server. It is configured during process setup.
func (m *JWTManager) AllowAudience(audience string) {
	audience = strings.TrimSpace(audience)
	if audience == "" || audience == m.audience {
		return
	}
	m.additionalAudiences[audience] = struct{}{}
}

var _ domainAuth.AccessTokenManager = (*JWTManager)(nil)

func (m *JWTManager) IssueAccessToken(claims domainAuth.AccessClaims) (string, error) {
	if claims.UserID == 0 || claims.SessionID == "" || claims.ExpiresAt.IsZero() || claims.IssuedAt.IsZero() {
		return "", domainAuth.ErrInvalidClaims
	}

	headerBytes, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", err
	}

	payloadBytes, err := json.Marshal(jwtClaims{
		Sub:       claimsSubject(claims.UserID),
		SID:       claims.SessionID,
		Typ:       claims.Type,
		Iss:       firstNonEmpty(claims.Issuer, m.issuer),
		Aud:       firstNonEmpty(claims.Audience, m.audience),
		ActorType: claims.ActorType,
		JTI:       claims.TokenID,
		Iat:       claims.IssuedAt.Unix(),
		Exp:       claims.ExpiresAt.Unix(),
	})
	if err != nil {
		return "", err
	}

	header := base64.RawURLEncoding.EncodeToString(headerBytes)
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signingInput := header + "." + payload
	signature := base64.RawURLEncoding.EncodeToString(m.sign(signingInput))
	return signingInput + "." + signature, nil
}

func (m *JWTManager) ParseAccessToken(token string, now time.Time) (*domainAuth.AccessClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, domainAuth.ErrInvalidClaims
	}

	signingInput := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, domainAuth.ErrInvalidClaims
	}
	if !hmac.Equal(signature, m.sign(signingInput)) {
		return nil, domainAuth.ErrInvalidClaims
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, domainAuth.ErrInvalidClaims
	}

	var payload jwtClaims
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, domainAuth.ErrInvalidClaims
	}

	if payload.Typ != "access" || payload.SID == "" || payload.Sub == "" {
		return nil, domainAuth.ErrInvalidClaims
	}
	if m.issuer != "" && payload.Iss != m.issuer {
		return nil, domainAuth.ErrInvalidClaims
	}
	if m.audience != "" && payload.Aud != m.audience {
		if _, ok := m.additionalAudiences[payload.Aud]; !ok {
			return nil, domainAuth.ErrInvalidClaims
		}
	}
	if now.Unix() >= payload.Exp {
		return nil, domainAuth.ErrInvalidClaims
	}

	userID, err := parseClaimsSubject(payload.Sub)
	if err != nil {
		return nil, domainAuth.ErrInvalidClaims
	}

	return &domainAuth.AccessClaims{
		UserID:    userID,
		SessionID: payload.SID,
		Type:      payload.Typ,
		Issuer:    payload.Iss,
		Audience:  payload.Aud,
		ActorType: payload.ActorType,
		TokenID:   payload.JTI,
		IssuedAt:  time.Unix(payload.Iat, 0),
		ExpiresAt: time.Unix(payload.Exp, 0),
	}, nil
}

func (m *JWTManager) sign(input string) []byte {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(input))
	return mac.Sum(nil)
}

func claimsSubject(userID uint) string {
	return strconv.FormatUint(uint64(userID), 10)
}

func parseClaimsSubject(subject string) (uint, error) {
	value, err := strconv.ParseUint(subject, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(value), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
