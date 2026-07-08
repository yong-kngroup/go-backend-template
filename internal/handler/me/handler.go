package me

import (
	"errors"

	"github.com/freeDog-wy/go-backend-template/internal/handler"
	handlerMiddleware "github.com/freeDog-wy/go-backend-template/internal/handler/middleware"
	svcAuth "github.com/freeDog-wy/go-backend-template/internal/usecase/auth"
	svcIdentity "github.com/freeDog-wy/go-backend-template/internal/usecase/identity"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	authSvc     *svcAuth.Service
	identitySvc *svcIdentity.Service
}

type UpdateProfileReq struct {
	Name string `json:"name" binding:"required,min=2,max=50"`
}

type ChangePasswordReq struct {
	CurrentPassword string `json:"current_password" binding:"required,min=6,max=100"`
	NewPassword     string `json:"new_password" binding:"required,min=6,max=100"`
}

type UserResponse struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func New(authSvc *svcAuth.Service, identitySvc *svcIdentity.Service) *Handler {
	return &Handler{
		authSvc:     authSvc,
		identitySvc: identitySvc,
	}
}

func (h *Handler) RegisterRoutes(route *gin.Engine) {
	group := route.Group("/api/v1")
	group.Use(handlerMiddleware.RequireAuth(h.authSvc))
	{
		group.GET("/me", h.GetProfile)
		group.PATCH("/me/profile", h.UpdateProfile)
		group.PATCH("/me/password", h.ChangePassword)
	}
}

func (h *Handler) GetProfile(c *gin.Context) {
	userID := c.GetUint(currentUserIDKey)
	user, err := h.identitySvc.GetByID(c.Request.Context(), userID)
	if err != nil {
		h.failIdentityError(c, err)
		return
	}
	handler.OK(c, fromUserResult(user))
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	var req UpdateProfileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, "INVALID_INPUT", err.Error())
		return
	}

	userID := c.GetUint(currentUserIDKey)
	user, err := h.identitySvc.UpdateProfile(c.Request.Context(), svcIdentity.UpdateProfileCmd{
		UserID: userID,
		Name:   req.Name,
	})
	if err != nil {
		h.failIdentityError(c, err)
		return
	}
	handler.OK(c, fromUserResult(user))
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var req ChangePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, "INVALID_INPUT", err.Error())
		return
	}

	userID := c.GetUint(currentUserIDKey)
	meta := handler.AuditMetaFromRequest(c)
	if err := h.authSvc.ChangePassword(c.Request.Context(), svcAuth.ChangePasswordCmd{
		UserID:          userID,
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
		IP:              meta.IP,
		UserAgent:       meta.UserAgent,
	}); err != nil {
		switch {
		case errors.Is(err, svcAuth.ErrInvalidCurrentPassword):
			handler.Fail(c, "INVALID_CURRENT_PASSWORD", "当前密码错误")
		default:
			handler.Fail(c, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	handler.OK(c, gin.H{"message": "密码修改成功"})
}

func (h *Handler) failIdentityError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, svcIdentity.ErrUserNotFound):
		handler.Fail(c, "USER_NOT_FOUND", "用户不存在")
	default:
		handler.Fail(c, "INTERNAL_ERROR", err.Error())
	}
}

func fromUserResult(user *svcIdentity.UserResult) *UserResponse {
	return &UserResponse{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
	}
}

const currentUserIDKey = handlerMiddleware.CurrentUserIDKey
