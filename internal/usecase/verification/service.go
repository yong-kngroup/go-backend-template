package verification

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	domainAudit "github.com/freeDog-wy/go-backend-template/internal/domain/audit"
	domainAuth "github.com/freeDog-wy/go-backend-template/internal/domain/auth"
	domainIdentity "github.com/freeDog-wy/go-backend-template/internal/domain/identity"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	domainVerification "github.com/freeDog-wy/go-backend-template/internal/domain/verification"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
)

type Service struct {
	tx             shared.TxManager
	userRepo       domainIdentity.Repository
	verifyRepo     domainVerification.Repository
	credentialRepo domainAuth.CredentialRepository
	pwdHasher      shared.PasswordHasher
	sessionStore   domainAuth.SessionStore
	eventBus       shared.EventBus
	logger         logger.Logger
}

func New(
	tx shared.TxManager,
	userRepo domainIdentity.Repository,
	verifyRepo domainVerification.Repository,
	credentialRepo domainAuth.CredentialRepository,
	pwdHasher shared.PasswordHasher,
	sessionStore domainAuth.SessionStore,
	eventBus shared.EventBus,
	logger logger.Logger,
) *Service {
	return &Service{
		tx:             tx,
		userRepo:       userRepo,
		verifyRepo:     verifyRepo,
		credentialRepo: credentialRepo,
		pwdHasher:      pwdHasher,
		sessionStore:   sessionStore,
		eventBus:       eventBus,
		logger:         logger,
	}
}

func (s *Service) ResendVerification(ctx context.Context, cmd ResendVerificationCmd) error {
	user, err := s.userRepo.FindByEmail(ctx, normalizeEmail(cmd.Email))
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return nil
		}
		return err
	}

	if user.IsEmailVerified() || !user.IsPendingVerification() {
		return nil
	}

	return s.tx.Do(ctx, func(ctx context.Context) error {
		verificationEvent, err := s.IssueEmailVerification(ctx, user)
		if err != nil {
			return err
		}
		return s.eventBus.Publish(ctx, verificationEvent)
	})
}

func (s *Service) ForgotPassword(ctx context.Context, cmd ForgotPasswordCmd) error {
	user, err := s.userRepo.FindByEmail(ctx, normalizeEmail(cmd.Email))
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return nil
		}
		return err
	}

	if !user.IsEmailVerified() || user.IsDeleted() {
		return nil
	}

	return s.tx.Do(ctx, func(ctx context.Context) error {
		resetEvent, err := s.IssuePasswordReset(ctx, user)
		if err != nil {
			return err
		}
		return s.eventBus.Publish(ctx, resetEvent)
	})
}

func (s *Service) VerifyEmail(ctx context.Context, cmd VerifyEmailCmd) error {
	now := time.Now()
	tokenHash := hashToken(cmd.Token)

	return s.tx.Do(ctx, func(ctx context.Context) error {
		token, err := s.verifyRepo.FindActiveByTokenHash(ctx, tokenHash, now)
		if err != nil {
			if errors.Is(err, shared.ErrNotFound) {
				return ErrInvalidVerificationToken
			}
			return err
		}

		user, err := s.userRepo.FindByID(ctx, token.GetUserID())
		if err != nil {
			if errors.Is(err, shared.ErrNotFound) {
				return ErrInvalidVerificationToken
			}
			return err
		}

		user.VerifyEmail()
		if err := user.Activate(); err != nil {
			return err
		}
		if err := s.userRepo.Update(ctx, user); err != nil {
			return err
		}

		if err := token.Consume(now); err != nil {
			return err
		}
		if err := s.verifyRepo.Update(ctx, token); err != nil {
			return err
		}
		s.publishAudit(ctx, domainAudit.LogRequested{
			ActorUserID: uintPtr(user.GetID()),
			TargetType:  "user",
			TargetID:    uintString(user.GetID()),
			Action:      domainAudit.ActionVerifyEmail,
			Result:      domainAudit.ResultSuccess,
			IP:          cmd.IP,
			UserAgent:   cmd.UserAgent,
		})
		return nil
	})
}

func (s *Service) ResetPassword(ctx context.Context, cmd ResetPasswordCmd) error {
	now := time.Now()
	tokenHash := hashToken(cmd.Token)

	var userID uint
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		token, err := s.verifyRepo.FindActivePasswordResetByTokenHash(ctx, tokenHash, now)
		if err != nil {
			if errors.Is(err, shared.ErrNotFound) {
				return ErrInvalidPasswordResetToken
			}
			return err
		}

		credential, err := s.credentialRepo.FindByUserID(ctx, token.GetUserID())
		if err != nil {
			if errors.Is(err, shared.ErrNotFound) {
				return ErrInvalidPasswordResetToken
			}
			return err
		}

		hashed, err := s.pwdHasher.Hash(cmd.Password)
		if err != nil {
			return err
		}
		if err := credential.ChangePassword(hashed, now); err != nil {
			return err
		}
		if err := s.credentialRepo.Update(ctx, credential); err != nil {
			return err
		}

		if err := token.Consume(now); err != nil {
			return err
		}
		if err := s.verifyRepo.UpdatePasswordReset(ctx, token); err != nil {
			return err
		}

		userID = token.GetUserID()
		return nil
	})
	if err != nil {
		return err
	}

	s.invalidateUserSession(ctx, userID)
	s.publishAudit(ctx, domainAudit.LogRequested{
		ActorUserID: uintPtr(userID),
		TargetType:  "user",
		TargetID:    uintString(userID),
		Action:      domainAudit.ActionResetPassword,
		Result:      domainAudit.ResultSuccess,
		IP:          cmd.IP,
		UserAgent:   cmd.UserAgent,
	})
	return nil
}

func (s *Service) IssueEmailVerification(ctx context.Context, user *domainIdentity.User) (domainVerification.EmailVerificationRequested, error) {
	rawToken, err := generateOpaqueToken()
	if err != nil {
		return domainVerification.EmailVerificationRequested{}, err
	}

	now := time.Now()
	if err := s.verifyRepo.InvalidateByUserID(ctx, user.GetID(), now); err != nil {
		return domainVerification.EmailVerificationRequested{}, err
	}

	token, err := domainVerification.NewEmailVerificationToken(
		user.GetID(),
		hashToken(rawToken),
		now.Add(emailVerificationTTL),
	)
	if err != nil {
		return domainVerification.EmailVerificationRequested{}, err
	}
	if err := s.verifyRepo.Create(ctx, token); err != nil {
		return domainVerification.EmailVerificationRequested{}, err
	}

	return domainVerification.EmailVerificationRequested{
		UserID: user.GetID(),
		Email:  user.GetEmail(),
		Token:  rawToken,
	}, nil
}

func (s *Service) IssuePasswordReset(ctx context.Context, user *domainIdentity.User) (domainVerification.PasswordResetRequested, error) {
	rawToken, err := generateOpaqueToken()
	if err != nil {
		return domainVerification.PasswordResetRequested{}, err
	}

	now := time.Now()
	if err := s.verifyRepo.InvalidatePasswordResetByUserID(ctx, user.GetID(), now); err != nil {
		return domainVerification.PasswordResetRequested{}, err
	}

	token, err := domainVerification.NewPasswordResetToken(
		user.GetID(),
		hashToken(rawToken),
		now.Add(passwordResetTTL),
	)
	if err != nil {
		return domainVerification.PasswordResetRequested{}, err
	}
	if err := s.verifyRepo.CreatePasswordReset(ctx, token); err != nil {
		return domainVerification.PasswordResetRequested{}, err
	}

	return domainVerification.PasswordResetRequested{
		UserID: user.GetID(),
		Email:  user.GetEmail(),
		Token:  rawToken,
	}, nil
}

func (s *Service) invalidateUserSession(ctx context.Context, userID uint) {
	if err := s.sessionStore.DeleteByUserID(ctx, userID); err != nil && s.logger != nil {
		s.logger.Error("invalidate user session failed", "user_id", userID, "error", err)
	}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
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
