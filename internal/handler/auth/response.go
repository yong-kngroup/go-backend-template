package auth

import (
	svcAuth "github.com/freeDog-wy/go-backend-template/internal/usecase/auth"
	svcIdentity "github.com/freeDog-wy/go-backend-template/internal/usecase/identity"
)

type UserResponse struct {
	ID     uint   `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Status string `json:"status"`
}

type AuthResponse struct {
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	User         *UserResponse `json:"user,omitempty"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

func FromUserResult(r *svcIdentity.UserResult) *UserResponse {
	if r == nil {
		return nil
	}

	return &UserResponse{
		ID:     r.ID,
		Name:   r.Name,
		Email:  r.Email,
		Status: r.Status,
	}
}

func FromAuthResult(r *svcAuth.AuthResult) *AuthResponse {
	return &AuthResponse{
		AccessToken:  r.AccessToken,
		RefreshToken: r.RefreshToken,
		User:         FromUserResult(r.User),
	}
}
