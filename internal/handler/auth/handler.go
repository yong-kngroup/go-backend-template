package auth

import (
	"errors"

	"github.com/freeDog-wy/go-backend-template/internal/handler"
	svcAuth "github.com/freeDog-wy/go-backend-template/internal/usecase/auth"
	svcAuthorization "github.com/freeDog-wy/go-backend-template/internal/usecase/authorization"
	svcIdentity "github.com/freeDog-wy/go-backend-template/internal/usecase/identity"
	svcVerification "github.com/freeDog-wy/go-backend-template/internal/usecase/verification"

	"github.com/gin-gonic/gin"
)

// Handler 认证 HTTP 处理器。
type Handler struct {
	authSvc          *svcAuth.Service
	authorizationSvc *svcAuthorization.Service
	identitySvc      *svcIdentity.Service
	verificationSvc  *svcVerification.Service
}

func New(
	authSvc *svcAuth.Service,
	authorizationSvc *svcAuthorization.Service,
	identitySvc *svcIdentity.Service,
	verificationSvc *svcVerification.Service,
) *Handler {
	return &Handler{
		authSvc:          authSvc,
		authorizationSvc: authorizationSvc,
		identitySvc:      identitySvc,
		verificationSvc:  verificationSvc,
	}
}

func (h *Handler) RegisterRoutes(route *gin.Engine) {
	group := route.Group("/api/v1")
	{
		group.POST("/auth/register", h.Register)
		group.POST("/auth/resend-verification", h.ResendVerification)
		group.POST("/auth/verify-email", h.VerifyEmail)
		group.POST("/auth/forgot-password", h.ForgotPassword)
		group.POST("/auth/reset-password", h.ResetPassword)
		group.POST("/auth/login", h.Login)
		group.POST("/auth/refresh", h.Refresh)
		group.POST("/auth/logout", h.Logout)
		group.POST("/admin/auth/login", h.AdminLogin)
	}
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, "INVALID_INPUT", err.Error())
		return
	}

	result, err := h.identitySvc.Register(c.Request.Context(), req.ToCommand())
	if err != nil {
		switch {
		case errors.Is(err, svcIdentity.ErrInvalidCaptcha):
			handler.Fail(c, "INVALID_CAPTCHA", "验证码错误")
		case errors.Is(err, svcIdentity.ErrEmailTaken):
			handler.Fail(c, "EMAIL_TAKEN", "邮箱已注册")
		default:
			handler.Fail(c, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	handler.OK(c, FromUserResult(result))
}

func (h *Handler) ResendVerification(c *gin.Context) {
	var req ResendVerificationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, "INVALID_INPUT", err.Error())
		return
	}

	if err := h.verificationSvc.ResendVerification(c.Request.Context(), req.ToCommand()); err != nil {
		handler.Fail(c, "INTERNAL_ERROR", err.Error())
		return
	}

	handler.OK(c, MessageResponse{Message: "如果账号存在且尚未验证，验证邮件已重新发送"})
}

func (h *Handler) VerifyEmail(c *gin.Context) {
	var req VerifyEmailReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, "INVALID_INPUT", err.Error())
		return
	}

	meta := handler.AuditMetaFromRequest(c)
	if err := h.verificationSvc.VerifyEmail(c.Request.Context(), svcVerification.VerifyEmailCmd{
		Token:     req.Token,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
	}); err != nil {
		switch {
		case errors.Is(err, svcVerification.ErrInvalidVerificationToken):
			handler.Fail(c, "INVALID_VERIFICATION_TOKEN", "验证链接无效或已过期")
		default:
			handler.Fail(c, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	handler.OK(c, MessageResponse{Message: "邮箱验证成功"})
}

func (h *Handler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, "INVALID_INPUT", err.Error())
		return
	}

	if err := h.verificationSvc.ForgotPassword(c.Request.Context(), req.ToCommand()); err != nil {
		handler.Fail(c, "INTERNAL_ERROR", err.Error())
		return
	}

	handler.OK(c, MessageResponse{Message: "如果账号存在且可重置，重置邮件已发送"})
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var req ResetPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, "INVALID_INPUT", err.Error())
		return
	}

	meta := handler.AuditMetaFromRequest(c)
	if err := h.verificationSvc.ResetPassword(c.Request.Context(), svcVerification.ResetPasswordCmd{
		Token:     req.Token,
		Password:  req.Password,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
	}); err != nil {
		switch {
		case errors.Is(err, svcVerification.ErrInvalidPasswordResetToken):
			handler.Fail(c, "INVALID_PASSWORD_RESET_TOKEN", "重置链接无效或已过期")
		default:
			handler.Fail(c, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	handler.OK(c, MessageResponse{Message: "密码重置成功"})
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, "INVALID_INPUT", err.Error())
		return
	}

	meta := handler.AuditMetaFromRequest(c)
	result, err := h.authSvc.Login(c.Request.Context(), svcAuth.LoginCmd{
		Email:     req.Email,
		Password:  req.Password,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
	})
	if err != nil {
		switch {
		case errors.Is(err, svcAuth.ErrInvalidCredentials):
			handler.Fail(c, "INVALID_CREDENTIALS", "邮箱或密码错误")
		case errors.Is(err, svcAuth.ErrEmailNotVerified):
			handler.Fail(c, "EMAIL_NOT_VERIFIED", "邮箱尚未验证")
		case errors.Is(err, svcAuth.ErrUserLocked):
			handler.Fail(c, "USER_LOCKED", "账号已锁定")
		case errors.Is(err, svcAuth.ErrUserBanned):
			handler.Fail(c, "USER_BANNED", "账号已禁用")
		default:
			handler.Fail(c, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	handler.OK(c, FromAuthResult(result))
}

func (h *Handler) Refresh(c *gin.Context) {
	var req RefreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, "INVALID_INPUT", err.Error())
		return
	}

	result, err := h.authSvc.Refresh(c.Request.Context(), req.ToCommand())
	if err != nil {
		switch {
		case errors.Is(err, svcAuth.ErrInvalidRefreshToken):
			handler.Fail(c, "INVALID_REFRESH_TOKEN", "refresh token 无效或已过期")
		case errors.Is(err, svcAuth.ErrEmailNotVerified):
			handler.Fail(c, "EMAIL_NOT_VERIFIED", "邮箱尚未验证")
		case errors.Is(err, svcAuth.ErrUserLocked):
			handler.Fail(c, "USER_LOCKED", "账号已锁定")
		case errors.Is(err, svcAuth.ErrUserBanned):
			handler.Fail(c, "USER_BANNED", "账号已禁用")
		default:
			handler.Fail(c, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	handler.OK(c, FromAuthResult(result))
}

func (h *Handler) Logout(c *gin.Context) {
	var req LogoutReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, "INVALID_INPUT", err.Error())
		return
	}

	meta := handler.AuditMetaFromRequest(c)
	if err := h.authSvc.Logout(c.Request.Context(), svcAuth.LogoutCmd{
		RefreshToken: req.RefreshToken,
		IP:           meta.IP,
		UserAgent:    meta.UserAgent,
	}); err != nil {
		switch {
		case errors.Is(err, svcAuth.ErrInvalidRefreshToken):
			handler.Fail(c, "INVALID_REFRESH_TOKEN", "refresh token 无效或已过期")
		default:
			handler.Fail(c, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	handler.OK(c, MessageResponse{Message: "已退出登录"})
}

func (h *Handler) AdminLogin(c *gin.Context) {
	var req LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, "INVALID_INPUT", err.Error())
		return
	}

	result, err := h.authSvc.Login(c.Request.Context(), req.ToCommand())
	if err != nil {
		switch {
		case errors.Is(err, svcAuth.ErrInvalidCredentials):
			handler.Fail(c, "INVALID_CREDENTIALS", "邮箱或密码错误")
		case errors.Is(err, svcAuth.ErrEmailNotVerified):
			handler.Fail(c, "EMAIL_NOT_VERIFIED", "邮箱尚未验证")
		case errors.Is(err, svcAuth.ErrUserLocked):
			handler.Fail(c, "USER_LOCKED", "账号已锁定")
		case errors.Is(err, svcAuth.ErrUserBanned):
			handler.Fail(c, "USER_BANNED", "账号已禁用")
		default:
			handler.Fail(c, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	if err := h.authorizationSvc.EnsureAdminAccess(c.Request.Context(), result.User.ID); err != nil {
		handler.Fail(c, "FORBIDDEN", "无管理后台访问权限")
		return
	}

	handler.OK(c, FromAuthResult(result))
}
