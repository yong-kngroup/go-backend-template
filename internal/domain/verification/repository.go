package verification

import (
	"context"
	"time"
)

type Repository interface {
	Create(ctx context.Context, token *EmailVerificationToken) error
	FindActiveByTokenHash(ctx context.Context, tokenHash string, now time.Time) (*EmailVerificationToken, error)
	InvalidateByUserID(ctx context.Context, userID uint, now time.Time) error
	Update(ctx context.Context, token *EmailVerificationToken) error
	DeleteExpiredEmailVerificationTokens(ctx context.Context, now time.Time) (int64, error)

	CreatePasswordReset(ctx context.Context, token *PasswordResetToken) error
	FindActivePasswordResetByTokenHash(ctx context.Context, tokenHash string, now time.Time) (*PasswordResetToken, error)
	InvalidatePasswordResetByUserID(ctx context.Context, userID uint, now time.Time) error
	UpdatePasswordReset(ctx context.Context, token *PasswordResetToken) error
	DeleteExpiredPasswordResetTokens(ctx context.Context, now time.Time) (int64, error)
}
