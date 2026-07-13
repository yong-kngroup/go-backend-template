package service_token

import (
	"errors"

	"github.com/freeDog-wy/go-backend-template/internal/handler"
	svcAuth "github.com/freeDog-wy/go-backend-template/internal/usecase/auth"
	"github.com/gin-gonic/gin"
)

type Handler struct{ issuer svcAuth.ServiceTokenIssuer }

func New(issuer svcAuth.ServiceTokenIssuer) *Handler { return &Handler{issuer: issuer} }

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.POST("/api/v1/auth/service-token", h.Issue)
}

func (h *Handler) Issue(c *gin.Context) {
	clientID, clientSecret, ok := c.Request.BasicAuth()
	if !ok {
		handler.Fail(c, "INVALID_SERVICE_CREDENTIALS", "service credentials are invalid")
		return
	}
	result, err := h.issuer.IssueServiceToken(c.Request.Context(), svcAuth.IssueServiceTokenCmd{ClientID: clientID, ClientSecret: clientSecret, IP: c.ClientIP(), UserAgent: c.Request.UserAgent()})
	if err != nil {
		if errors.Is(err, svcAuth.ErrInvalidServiceCredential) {
			handler.Fail(c, "INVALID_SERVICE_CREDENTIALS", "service credentials are invalid")
			return
		}
		handler.Fail(c, "INTERNAL_ERROR", "service token cannot be issued")
		return
	}
	handler.OK(c, response{AccessToken: result.AccessToken, TokenType: "Bearer", ExpiresIn: result.ExpiresIn})
}

type response struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}
