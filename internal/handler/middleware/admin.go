package middleware

import (
	"github.com/freeDog-wy/go-backend-template/internal/handler"
	svcAuth "github.com/freeDog-wy/go-backend-template/internal/usecase/auth"
	svcAuthorization "github.com/freeDog-wy/go-backend-template/internal/usecase/authorization"

	"github.com/gin-gonic/gin"
)

func RequireAdmin(authSvc *svcAuth.Service, authorizationSvc *svcAuthorization.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := authenticateRequest(c, authSvc)
		if !ok {
			return
		}

		if err := authorizationSvc.EnsureAdminAccess(c.Request.Context(), userID); err != nil {
			handler.Fail(c, "FORBIDDEN", "无管理后台访问权限")
			c.Abort()
			return
		}

		c.Set(CurrentUserIDKey, userID)
		c.Next()
	}
}

func RequirePermission(authSvc *svcAuth.Service, authorizationSvc *svcAuthorization.Service, code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := authenticateRequest(c, authSvc)
		if !ok {
			return
		}

		if err := authorizationSvc.EnsureAdminAccess(c.Request.Context(), userID); err != nil {
			handler.Fail(c, "FORBIDDEN", "无管理后台访问权限")
			c.Abort()
			return
		}

		allowed, err := authorizationSvc.HasPermission(c.Request.Context(), userID, code)
		if err != nil {
			handler.Fail(c, "INTERNAL_ERROR", err.Error())
			c.Abort()
			return
		}
		if !allowed {
			handler.Fail(c, "FORBIDDEN", "缺少所需权限")
			c.Abort()
			return
		}

		c.Set(CurrentUserIDKey, userID)
		c.Next()
	}
}
