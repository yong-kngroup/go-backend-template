package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	domainAudit "github.com/freeDog-wy/go-backend-template/internal/domain/audit"
	domainAuth "github.com/freeDog-wy/go-backend-template/internal/domain/auth"
	domainIdentity "github.com/freeDog-wy/go-backend-template/internal/domain/identity"
	domainMCP "github.com/freeDog-wy/go-backend-template/internal/domain/mcp"
	domainShared "github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
)

type ServiceTokenService struct {
	accountRepo  domainMCP.ServiceAccountRepository
	userRepo     domainIdentity.Repository
	sessionStore domainAuth.SessionStore
	hasher       domainShared.PasswordHasher
	tokenManager domainAuth.AccessTokenManager
	eventBus     domainShared.EventBus
	logger       logger.Logger
	issuer       string
	audience     string
	ttl          time.Duration
}

func NewServiceTokenService(accountRepo domainMCP.ServiceAccountRepository, userRepo domainIdentity.Repository, sessionStore domainAuth.SessionStore, hasher domainShared.PasswordHasher, tokenManager domainAuth.AccessTokenManager, eventBus domainShared.EventBus, log logger.Logger, issuer, audience string, ttl time.Duration) *ServiceTokenService {
	return &ServiceTokenService{accountRepo: accountRepo, userRepo: userRepo, sessionStore: sessionStore, hasher: hasher, tokenManager: tokenManager, eventBus: eventBus, logger: log, issuer: issuer, audience: audience, ttl: ttl}
}

func (s *ServiceTokenService) IssueServiceToken(ctx context.Context, cmd IssueServiceTokenCmd) (*ServiceTokenResult, error) {
	clientID := strings.TrimSpace(cmd.ClientID)
	if clientID == "" || strings.TrimSpace(cmd.ClientSecret) == "" {
		return nil, ErrInvalidServiceCredential
	}
	account, err := s.accountRepo.FindByClientID(ctx, clientID)
	if err != nil || !account.IsEnabled() || !s.matchesServiceSecret(account, cmd.ClientSecret, time.Now()) {
		s.publishAudit(ctx, nil, clientID, "", domainAudit.ResultFailure, cmd.IP, cmd.UserAgent)
		return nil, ErrInvalidServiceCredential
	}
	user, err := s.userRepo.FindByID(ctx, account.GetUserID())
	if err != nil || validateUserForLogin(user) != nil {
		s.publishAudit(ctx, uintPtr(account.GetUserID()), clientID, "", domainAudit.ResultFailure, cmd.IP, cmd.UserAgent)
		return nil, ErrInvalidServiceCredential
	}
	if s.ttl <= 0 {
		return nil, errors.New("service token ttl must be positive")
	}
	now := time.Now()
	if err := s.sessionStore.DeleteByUserID(ctx, user.GetID()); err != nil {
		return nil, err
	}
	sessionID, err := randomServiceToken(18)
	if err != nil {
		return nil, err
	}
	proof, err := randomServiceToken(32)
	if err != nil {
		return nil, err
	}
	expiresAt := now.Add(s.ttl)
	session, err := domainAuth.NewRefreshSession(sessionID, user.GetID(), hashServiceToken(proof), expiresAt)
	if err != nil {
		return nil, err
	}
	if err := s.sessionStore.Save(ctx, session); err != nil {
		return nil, err
	}
	accessToken, err := s.tokenManager.IssueAccessToken(domainAuth.AccessClaims{UserID: user.GetID(), SessionID: sessionID, Type: "access", Issuer: s.issuer, Audience: s.audience, ActorType: "service", TokenID: sessionID, IssuedAt: now, ExpiresAt: expiresAt})
	if err != nil {
		return nil, err
	}
	s.publishAudit(ctx, uintPtr(user.GetID()), clientID, sessionID, domainAudit.ResultSuccess, cmd.IP, cmd.UserAgent)
	return &ServiceTokenResult{AccessToken: accessToken, ExpiresIn: int(s.ttl.Seconds())}, nil
}

func (s *ServiceTokenService) matchesServiceSecret(account *domainMCP.ServiceAccount, secret string, now time.Time) bool {
	return s.hasher.Verify(secret, account.GetClientSecretHash()) || (account.PreviousSecretActive(now) && s.hasher.Verify(secret, account.GetPreviousClientSecretHash()))
}

func (s *ServiceTokenService) publishAudit(ctx context.Context, actorUserID *uint, clientID, tokenID, result, ip, userAgent string) {
	if s.eventBus == nil {
		return
	}
	err := s.eventBus.Publish(ctx, domainAudit.LogRequested{ActorUserID: actorUserID, TargetType: "mcp_service_account", TargetID: clientID, Action: "mcp_service_token_issued", Result: result, IP: ip, UserAgent: userAgent, Metadata: map[string]any{"actor_type": "service", "actor_id": clientID, "jti": tokenID}})
	if err != nil && s.logger != nil {
		s.logger.Error("publish mcp service token audit failed", "client_id", clientID, "error", err)
	}
}

func randomServiceToken(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func hashServiceToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
