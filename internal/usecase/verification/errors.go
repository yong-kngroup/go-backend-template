package verification

import "errors"

var ErrInvalidVerificationToken = errors.New("invalid verification token")

var ErrInvalidPasswordResetToken = errors.New("invalid password reset token")
