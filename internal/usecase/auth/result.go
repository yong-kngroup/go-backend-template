package auth

import svcIdentity "github.com/freeDog-wy/go-backend-template/internal/usecase/identity"

type AuthResult struct {
	AccessToken  string
	RefreshToken string
	User         *svcIdentity.UserResult
}

type AccessIdentity struct {
	UserID    uint
	SessionID string
}

type ServiceTokenResult struct {
	AccessToken string
	ExpiresIn   int
}
