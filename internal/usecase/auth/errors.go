package auth

import "errors"

var (
	ErrInvalidCredentials       = errors.New("invalid credentials")
	ErrEmailNotVerified         = errors.New("email not verified")
	ErrUserLocked               = errors.New("user locked")
	ErrUserBanned               = errors.New("user banned")
	ErrUserDeleted              = errors.New("user deleted")
	ErrInvalidRefreshToken      = errors.New("invalid refresh token")
	ErrInvalidAccessToken       = errors.New("invalid access token")
	ErrInvalidServiceCredential = errors.New("invalid service credential")
	ErrAuthenticationMissing    = errors.New("missing authentication")
	ErrInvalidCurrentPassword   = errors.New("invalid current password")
)
