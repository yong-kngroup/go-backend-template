package admin_user

import (
	"errors"
	"strconv"

	domainAuthorization "github.com/freeDog-wy/go-backend-template/internal/domain/authorization"
	domainIdentity "github.com/freeDog-wy/go-backend-template/internal/domain/identity"
	"github.com/freeDog-wy/go-backend-template/internal/handler"
	handlerMiddleware "github.com/freeDog-wy/go-backend-template/internal/handler/middleware"
	svcAuth "github.com/freeDog-wy/go-backend-template/internal/usecase/auth"
	svcAuthorization "github.com/freeDog-wy/go-backend-template/internal/usecase/authorization"
	svcIdentity "github.com/freeDog-wy/go-backend-template/internal/usecase/identity"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	authSvc          *svcAuth.Service
	authorizationSvc *svcAuthorization.Service
	identitySvc      *svcIdentity.Service
}

type UpdateStatusReq struct {
	Status string `json:"status" binding:"required,oneof=active locked banned"`
}

type ReplaceRolesReq struct {
	RoleIDs []uint `json:"role_ids"`
}

type CreateUserReq struct {
	Name     string `json:"name" binding:"required,min=2,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6,max=100"`
	RoleIDs  []uint `json:"role_ids"`
}

type UserResponse struct {
	ID     uint                           `json:"id"`
	Name   string                         `json:"name"`
	Email  string                         `json:"email"`
	Status string                         `json:"status"`
	Roles  []*svcAuthorization.RoleResult `json:"roles"`
}

func New(
	authSvc *svcAuth.Service,
	authorizationSvc *svcAuthorization.Service,
	identitySvc *svcIdentity.Service,
) *Handler {
	return &Handler{
		authSvc:          authSvc,
		authorizationSvc: authorizationSvc,
		identitySvc:      identitySvc,
	}
}

func (h *Handler) RegisterRoutes(route *gin.Engine) {
	admin := route.Group("/api/v1/admin")
	{
		admin.GET("/users", handlerMiddleware.RequirePermission(h.authSvc, h.authorizationSvc, "user.read"), h.ListUsers)
		admin.POST("/users", handlerMiddleware.RequirePermission(h.authSvc, h.authorizationSvc, "user.write"), h.CreateUser)
		admin.PATCH("/users/:id/status", handlerMiddleware.RequirePermission(h.authSvc, h.authorizationSvc, "user.ban"), h.UpdateStatus)
		admin.PUT("/users/:id/roles", handlerMiddleware.RequirePermission(h.authSvc, h.authorizationSvc, "user.write"), h.ReplaceRoles)
	}
}

func (h *Handler) ListUsers(c *gin.Context) {
	var query handler.PageQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		handler.Fail(c, "INVALID_INPUT", err.Error())
		return
	}

	users, pageResult, err := h.identitySvc.List(c.Request.Context(), svcIdentity.ListUsersCmd{
		Page: query.ToDomain(),
	})
	if err != nil {
		handler.Fail(c, "INTERNAL_ERROR", err.Error())
		return
	}

	responses := make([]*UserResponse, 0, len(users))
	for _, user := range users {
		response, err := h.buildUserResponse(c, user)
		if err != nil {
			handler.Fail(c, "INTERNAL_ERROR", err.Error())
			return
		}
		responses = append(responses, response)
	}
	handler.OKPage(c, responses, handler.MetaFromPageResult(pageResult))
}

func (h *Handler) CreateUser(c *gin.Context) {
	var req CreateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, "INVALID_INPUT", err.Error())
		return
	}

	user, err := h.identitySvc.CreateAdminUser(c.Request.Context(), svcIdentity.CreateAdminUserCmd{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
		RoleIDs:  req.RoleIDs,
	})
	if err != nil {
		switch {
		case errors.Is(err, svcIdentity.ErrEmailTaken):
			handler.Fail(c, "EMAIL_TAKEN", "邮箱已注册")
		case errors.Is(err, domainAuthorization.ErrRoleNotFound):
			handler.Fail(c, "ROLE_NOT_FOUND", "角色不存在")
		case errors.Is(err, domainIdentity.ErrInvalidUserData):
			handler.Fail(c, "INVALID_INPUT", "用户信息不合法")
		default:
			handler.Fail(c, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response, err := h.buildUserResponse(c, user)
	if err != nil {
		handler.Fail(c, "INTERNAL_ERROR", err.Error())
		return
	}
	handler.OK(c, response)
}

func (h *Handler) UpdateStatus(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		handler.Fail(c, "INVALID_INPUT", "非法用户 ID")
		return
	}

	var req UpdateStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, "INVALID_INPUT", err.Error())
		return
	}

	meta := handler.AuditMetaFromRequest(c)
	user, err := h.identitySvc.UpdateStatus(c.Request.Context(), svcIdentity.UpdateStatusCmd{
		UserID:      uint(userID),
		Status:      req.Status,
		ActorUserID: handlerMiddleware.CurrentUserID(c),
		IP:          meta.IP,
		UserAgent:   meta.UserAgent,
	})
	if err != nil {
		handler.Fail(c, "INTERNAL_ERROR", err.Error())
		return
	}
	response, err := h.buildUserResponse(c, user)
	if err != nil {
		handler.Fail(c, "INTERNAL_ERROR", err.Error())
		return
	}
	handler.OK(c, response)
}

func (h *Handler) ReplaceRoles(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		handler.Fail(c, "INVALID_INPUT", "非法用户 ID")
		return
	}

	var req ReplaceRolesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, "INVALID_INPUT", err.Error())
		return
	}

	actorUserID := handlerMiddleware.CurrentUserID(c)
	meta := handler.AuditMetaFromRequest(c)
	if err := h.authorizationSvc.ReplaceUserRoles(c.Request.Context(), svcAuthorization.ReplaceUserRolesCmd{
		UserID:      uint(userID),
		RoleIDs:     req.RoleIDs,
		ActorUserID: actorUserID,
		IP:          meta.IP,
		UserAgent:   meta.UserAgent,
	}); err != nil {
		switch {
		case errors.Is(err, domainAuthorization.ErrRoleNotFound):
			handler.Fail(c, "ROLE_NOT_FOUND", "角色不存在")
		default:
			handler.Fail(c, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	roles, err := h.authorizationSvc.ListUserRoles(c.Request.Context(), uint(userID))
	if err != nil {
		handler.Fail(c, "INTERNAL_ERROR", err.Error())
		return
	}
	handler.OK(c, gin.H{
		"user_id": uint(userID),
		"roles":   roles,
	})
}

func (h *Handler) buildUserResponse(c *gin.Context, user *svcIdentity.UserResult) (*UserResponse, error) {
	roles, err := h.authorizationSvc.ListUserRoles(c.Request.Context(), user.ID)
	if err != nil {
		return nil, err
	}
	return &UserResponse{
		ID:     user.ID,
		Name:   user.Name,
		Email:  user.Email,
		Status: user.Status,
		Roles:  roles,
	}, nil
}
