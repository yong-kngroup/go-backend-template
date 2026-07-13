package auth

import "context"

// AccessAuthenticator validates an access token and resolves its active identity.
type AccessAuthenticator interface {
	AuthenticateAccessToken(ctx context.Context, token string) (*AccessIdentity, error)
}

// AuthenticationService manages login and refresh-session lifecycle operations.
type AuthenticationService interface {
	Login(ctx context.Context, cmd LoginCmd) (*AuthResult, error)
	Refresh(ctx context.Context, cmd RefreshCmd) (*AuthResult, error)
	Logout(ctx context.Context, cmd LogoutCmd) error
}

// PasswordService changes an authenticated user's password.
type PasswordService interface {
	ChangePassword(ctx context.Context, cmd ChangePasswordCmd) error
}

type ServiceTokenIssuer interface {
	IssueServiceToken(ctx context.Context, cmd IssueServiceTokenCmd) (*ServiceTokenResult, error)
}

var (
	_ AccessAuthenticator   = (*Service)(nil)
	_ AuthenticationService = (*Service)(nil)
	_ PasswordService       = (*Service)(nil)
	_ ServiceTokenIssuer    = (*ServiceTokenService)(nil)
)
