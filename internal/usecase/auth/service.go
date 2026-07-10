package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	domainAudit "github.com/freeDog-wy/go-backend-template/internal/domain/audit"
	domainAuth "github.com/freeDog-wy/go-backend-template/internal/domain/auth"
	domainIdentity "github.com/freeDog-wy/go-backend-template/internal/domain/identity"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	svcIdentity "github.com/freeDog-wy/go-backend-template/internal/usecase/identity"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var authTracer = otel.Tracer("github.com/freeDog-wy/go-backend-template/internal/usecase/auth")

type Service struct {
	userRepo        domainIdentity.Repository
	credentialRepo  domainAuth.CredentialRepository
	sessionStore    domainAuth.SessionStore
	passwordHasher  shared.PasswordHasher
	tokenManager    domainAuth.AccessTokenManager
	eventBus        shared.EventBus
	logger          logger.Logger
	issuer          string
	audience        string
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

func New(
	userRepo domainIdentity.Repository,
	credentialRepo domainAuth.CredentialRepository,
	sessionStore domainAuth.SessionStore,
	passwordHasher shared.PasswordHasher,
	tokenManager domainAuth.AccessTokenManager,
	eventBus shared.EventBus,
	logger logger.Logger,
	issuer string,
	audience string,
	accessTokenTTL time.Duration,
	refreshTokenTTL time.Duration,
) *Service {
	return &Service{
		userRepo:        userRepo,
		credentialRepo:  credentialRepo,
		sessionStore:    sessionStore,
		passwordHasher:  passwordHasher,
		tokenManager:    tokenManager,
		eventBus:        eventBus,
		logger:          logger,
		issuer:          issuer,
		audience:        audience,
		accessTokenTTL:  accessTokenTTL,
		refreshTokenTTL: refreshTokenTTL,
	}
}

func (s *Service) Login(ctx context.Context, cmd LoginCmd) (result *AuthResult, err error) {
	ctx, span := authTracer.Start(ctx, "auth.login")
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
			if result != nil && result.User != nil {
				span.SetAttributes(attribute.Int64("app.user.id", int64(result.User.ID)))
			}
		}
		span.End()
	}()

	email := strings.ToLower(strings.TrimSpace(cmd.Email))
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			s.publishAudit(ctx, domainAudit.LogRequested{
				TargetType: "user_email",
				TargetID:   email,
				Action:     domainAudit.ActionLogin,
				Result:     domainAudit.ResultFailure,
				IP:         cmd.IP,
				UserAgent:  cmd.UserAgent,
				Metadata: map[string]any{
					"reason": "invalid_credentials",
				},
			})
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if err := validateUserForLogin(user); err != nil {
		s.publishAudit(ctx, domainAudit.LogRequested{
			TargetType: "user",
			TargetID:   uintString(user.GetID()),
			Action:     domainAudit.ActionLogin,
			Result:     domainAudit.ResultFailure,
			IP:         cmd.IP,
			UserAgent:  cmd.UserAgent,
			Metadata: map[string]any{
				"reason": err.Error(),
			},
		})
		return nil, err
	}

	credential, err := s.credentialRepo.FindByUserID(ctx, user.GetID())
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if !s.passwordHasher.Verify(cmd.Password, credential.GetPasswordHash()) {
		s.publishAudit(ctx, domainAudit.LogRequested{
			TargetType: "user",
			TargetID:   uintString(user.GetID()),
			Action:     domainAudit.ActionLogin,
			Result:     domainAudit.ResultFailure,
			IP:         cmd.IP,
			UserAgent:  cmd.UserAgent,
			Metadata: map[string]any{
				"reason": "invalid_credentials",
			},
		})
		return nil, ErrInvalidCredentials
	}

	now := time.Now()
	user.RecordLogin(now)
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}
	result, err = s.issueTokens(ctx, user, now)
	if err != nil {
		return nil, err
	}
	s.publishAudit(ctx, domainAudit.LogRequested{
		TargetType: "user",
		TargetID:   uintString(user.GetID()),
		Action:     domainAudit.ActionLogin,
		Result:     domainAudit.ResultSuccess,
		IP:         cmd.IP,
		UserAgent:  cmd.UserAgent,
	})
	return result, nil
}

func (s *Service) Refresh(ctx context.Context, cmd RefreshCmd) (result *AuthResult, err error) {
	ctx, span := authTracer.Start(ctx, "auth.refresh")
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
			if result != nil && result.User != nil {
				span.SetAttributes(attribute.Int64("app.user.id", int64(result.User.ID)))
			}
		}
		span.End()
	}()

	sessionID, secret, err := parseRefreshToken(cmd.RefreshToken)
	if err != nil {
		return nil, ErrInvalidRefreshToken
	}

	session, err := s.sessionStore.FindByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, err
	}

	now := time.Now()
	if session.IsExpired(now) || session.GetTokenHash() != hashToken(secret) {
		_ = s.sessionStore.DeleteByID(ctx, sessionID)
		return nil, ErrInvalidRefreshToken
	}

	user, err := s.userRepo.FindByID(ctx, session.GetUserID())
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, err
	}

	if err := validateUserForLogin(user); err != nil {
		return nil, err
	}

	_ = s.sessionStore.DeleteByID(ctx, sessionID)
	result, err = s.issueTokens(ctx, user, now)
	return result, err
}

func (s *Service) Logout(ctx context.Context, cmd LogoutCmd) error {
	sessionID, _, err := parseRefreshToken(cmd.RefreshToken)
	if err != nil {
		return ErrInvalidRefreshToken
	}
	session, err := s.sessionStore.FindByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return ErrInvalidRefreshToken
		}
		return err
	}
	if err := s.sessionStore.DeleteByID(ctx, sessionID); err != nil {
		return err
	}
	s.publishAudit(ctx, domainAudit.LogRequested{
		ActorUserID: uintPtr(session.GetUserID()),
		TargetType:  "user",
		TargetID:    uintString(session.GetUserID()),
		Action:      domainAudit.ActionLogout,
		Result:      domainAudit.ResultSuccess,
		IP:          cmd.IP,
		UserAgent:   cmd.UserAgent,
	})
	return nil
}

func (s *Service) ChangePassword(ctx context.Context, cmd ChangePasswordCmd) error {
	credential, err := s.credentialRepo.FindByUserID(ctx, cmd.UserID)
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return ErrInvalidCurrentPassword
		}
		return err
	}

	if !s.passwordHasher.Verify(cmd.CurrentPassword, credential.GetPasswordHash()) {
		s.publishAudit(ctx, domainAudit.LogRequested{
			ActorUserID: uintPtr(cmd.UserID),
			TargetType:  "user",
			TargetID:    uintString(cmd.UserID),
			Action:      domainAudit.ActionChangePassword,
			Result:      domainAudit.ResultFailure,
			IP:          cmd.IP,
			UserAgent:   cmd.UserAgent,
			Metadata: map[string]any{
				"reason": "invalid_current_password",
			},
		})
		return ErrInvalidCurrentPassword
	}

	hashed, err := s.passwordHasher.Hash(cmd.NewPassword)
	if err != nil {
		return err
	}
	if err := credential.ChangePassword(hashed, time.Now()); err != nil {
		return err
	}
	if err := s.credentialRepo.Update(ctx, credential); err != nil {
		return err
	}

	if err := s.sessionStore.DeleteByUserID(ctx, cmd.UserID); err != nil && s.logger != nil {
		s.logger.Error("invalidate user session failed after password change", "user_id", cmd.UserID, "error", err)
	}
	s.publishAudit(ctx, domainAudit.LogRequested{
		ActorUserID: uintPtr(cmd.UserID),
		TargetType:  "user",
		TargetID:    uintString(cmd.UserID),
		Action:      domainAudit.ActionChangePassword,
		Result:      domainAudit.ResultSuccess,
		IP:          cmd.IP,
		UserAgent:   cmd.UserAgent,
	})
	return nil
}

// ParseAccessToken only validates and parses the JWT itself.
// It does not verify whether the backing session is still active.
// Use AuthenticateAccessToken for request authentication.
func (s *Service) ParseAccessToken(token string) (*AccessIdentity, error) {
	claims, err := s.tokenManager.ParseAccessToken(token, time.Now())
	if err != nil {
		return nil, ErrInvalidAccessToken
	}

	return &AccessIdentity{
		UserID:    claims.UserID,
		SessionID: claims.SessionID,
	}, nil
}

// AuthenticateAccessToken validates the JWT and confirms the backing session is still active.
func (s *Service) AuthenticateAccessToken(ctx context.Context, token string) (identity *AccessIdentity, err error) {
	ctx, span := authTracer.Start(ctx, "auth.authenticate_access_token")
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
			if identity != nil {
				span.SetAttributes(attribute.Int64("app.user.id", int64(identity.UserID)))
			}
		}
		span.End()
	}()

	claims, err := s.tokenManager.ParseAccessToken(token, time.Now())
	if err != nil {
		return nil, ErrInvalidAccessToken
	}

	session, err := s.sessionStore.FindByID(ctx, claims.SessionID)
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return nil, ErrInvalidAccessToken
		}
		return nil, err
	}

	now := time.Now()
	if session.IsExpired(now) {
		_ = s.sessionStore.DeleteByID(ctx, claims.SessionID)
		return nil, ErrInvalidAccessToken
	}
	if session.GetUserID() != claims.UserID {
		_ = s.sessionStore.DeleteByID(ctx, claims.SessionID)
		return nil, ErrInvalidAccessToken
	}

	identity = &AccessIdentity{
		UserID:    claims.UserID,
		SessionID: claims.SessionID,
	}
	return identity, nil
}

func (s *Service) issueTokens(ctx context.Context, user *domainIdentity.User, now time.Time) (*AuthResult, error) {
	_ = s.sessionStore.DeleteByUserID(ctx, user.GetID())

	sessionID, err := randomEncodedToken(18)
	if err != nil {
		return nil, err
	}
	rawSecret, err := randomEncodedToken(32)
	if err != nil {
		return nil, err
	}

	session, err := domainAuth.NewRefreshSession(
		sessionID,
		user.GetID(),
		hashToken(rawSecret),
		now.Add(s.refreshTokenTTL),
	)
	if err != nil {
		return nil, err
	}
	if err := s.sessionStore.Save(ctx, session); err != nil {
		return nil, err
	}

	accessToken, err := s.tokenManager.IssueAccessToken(domainAuth.AccessClaims{
		UserID:    user.GetID(),
		SessionID: sessionID,
		Type:      "access",
		Issuer:    s.issuer,
		Audience:  s.audience,
		IssuedAt:  now,
		ExpiresAt: now.Add(s.accessTokenTTL),
	})
	if err != nil {
		return nil, err
	}

	if s.logger != nil {
		s.logger.Info("user authenticated", "user_id", user.GetID(), "session_id", sessionID)
	}

	return &AuthResult{
		AccessToken:  accessToken,
		RefreshToken: buildRefreshToken(sessionID, rawSecret),
		User:         svcIdentity.FromEntity(user),
	}, nil
}

func validateUserForLogin(user *domainIdentity.User) error {
	switch {
	case user.IsPendingVerification() || !user.IsEmailVerified():
		return ErrEmailNotVerified
	case user.IsLocked():
		return ErrUserLocked
	case user.IsBanned():
		return ErrUserBanned
	case user.IsDeleted():
		return ErrUserDeleted
	default:
		return nil
	}
}

func buildRefreshToken(sessionID, rawSecret string) string {
	return sessionID + "." + rawSecret
}

func parseRefreshToken(token string) (string, string, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", ErrInvalidRefreshToken
	}
	return parts[0], parts[1], nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomEncodedToken(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (s *Service) publishAudit(ctx context.Context, evt domainAudit.LogRequested) {
	if s.eventBus == nil {
		return
	}
	if err := s.eventBus.Publish(ctx, evt); err != nil && s.logger != nil {
		s.logger.Error("publish audit event failed", "action", evt.Action, "error", err)
	}
}

func uintPtr(value uint) *uint {
	return &value
}

func uintString(value uint) string {
	return strconv.FormatUint(uint64(value), 10)
}
