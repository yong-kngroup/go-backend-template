package middleware

import (
	"strings"

	"github.com/freeDog-wy/go-backend-template/internal/handler"
	svcAuth "github.com/freeDog-wy/go-backend-template/internal/usecase/auth"

	"github.com/gin-gonic/gin"
)

func RequireAuth(authSvc *svcAuth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := authenticateRequest(c, authSvc)
		if !ok {
			return
		}
		c.Set(CurrentUserIDKey, userID)
		c.Next()
	}
}

func authenticateRequest(c *gin.Context, authSvc *svcAuth.Service) (uint, bool) {
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if len(authHeader) < len("Bearer ")+1 || !strings.EqualFold(authHeader[:len("Bearer ")], "Bearer ") {
		handler.Fail(c, "UNAUTHORIZED", "缺少 access token")
		c.Abort()
		return 0, false
	}

	token := strings.TrimSpace(authHeader[len("Bearer "):])
	identity, err := authSvc.ParseAccessToken(token)
	if err != nil {
		handler.Fail(c, "UNAUTHORIZED", "access token 无效或已过期")
		c.Abort()
		return 0, false
	}

	return identity.UserID, true
}
