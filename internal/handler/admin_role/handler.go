package admin_role

import (
	"errors"
	"strconv"

	domainAuthorization "github.com/freeDog-wy/go-backend-template/internal/domain/authorization"
	"github.com/freeDog-wy/go-backend-template/internal/handler"
	handlerMiddleware "github.com/freeDog-wy/go-backend-template/internal/handler/middleware"
	svcAuth "github.com/freeDog-wy/go-backend-template/internal/usecase/auth"
	svcAuthorization "github.com/freeDog-wy/go-backend-template/internal/usecase/authorization"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	authSvc          *svcAuth.Service
	authorizationSvc *svcAuthorization.Service
}

type CreateRoleReq struct {
	Code            string   `json:"code" binding:"required,min=2,max=100"`
	Name            string   `json:"name" binding:"required,min=2,max=100"`
	Description     string   `json:"description"`
	PermissionCodes []string `json:"permission_codes"`
}

type UpdateRoleReq struct {
	Name            string   `json:"name" binding:"required,min=2,max=100"`
	Description     string   `json:"description"`
	PermissionCodes []string `json:"permission_codes"`
}

func New(authSvc *svcAuth.Service, authorizationSvc *svcAuthorization.Service) *Handler {
	return &Handler{
		authSvc:          authSvc,
		authorizationSvc: authorizationSvc,
	}
}

func (h *Handler) RegisterRoutes(route *gin.Engine) {
	admin := route.Group("/api/v1/admin")
	{
		admin.GET("/roles", handlerMiddleware.RequirePermission(h.authSvc, h.authorizationSvc, "role.read"), h.ListRoles)
		admin.POST("/roles", handlerMiddleware.RequirePermission(h.authSvc, h.authorizationSvc, "role.write"), h.CreateRole)
		admin.PATCH("/roles/:id", handlerMiddleware.RequirePermission(h.authSvc, h.authorizationSvc, "role.write"), h.UpdateRole)
		admin.GET("/permissions", handlerMiddleware.RequirePermission(h.authSvc, h.authorizationSvc, "permission.read"), h.ListPermissions)
	}
}

func (h *Handler) ListRoles(c *gin.Context) {
	var query handler.PageQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		handler.Fail(c, "INVALID_INPUT", err.Error())
		return
	}

	roles, pageResult, err := h.authorizationSvc.ListRoles(c.Request.Context(), svcAuthorization.ListRolesCmd{
		Page: query.ToDomain(),
	})
	if err != nil {
		handler.Fail(c, "INTERNAL_ERROR", err.Error())
		return
	}
	handler.OKPage(c, roles, handler.MetaFromPageResult(pageResult))
}

func (h *Handler) CreateRole(c *gin.Context) {
	var req CreateRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, "INVALID_INPUT", err.Error())
		return
	}

	role, err := h.authorizationSvc.CreateRole(c.Request.Context(), svcAuthorization.CreateRoleCmd{
		Code:            req.Code,
		Name:            req.Name,
		Description:     req.Description,
		PermissionCodes: req.PermissionCodes,
	})
	if err != nil {
		handler.Fail(c, "INTERNAL_ERROR", err.Error())
		return
	}
	handler.OK(c, role)
}

func (h *Handler) UpdateRole(c *gin.Context) {
	roleID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		handler.Fail(c, "INVALID_INPUT", "非法角色 ID")
		return
	}

	var req UpdateRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, "INVALID_INPUT", err.Error())
		return
	}

	role, err := h.authorizationSvc.UpdateRole(c.Request.Context(), svcAuthorization.UpdateRoleCmd{
		RoleID:          uint(roleID),
		Name:            req.Name,
		Description:     req.Description,
		PermissionCodes: req.PermissionCodes,
	})
	if err != nil {
		switch {
		case errors.Is(err, domainAuthorization.ErrRoleNotFound):
			handler.Fail(c, "ROLE_NOT_FOUND", "角色不存在")
		default:
			handler.Fail(c, "INTERNAL_ERROR", err.Error())
		}
		return
	}
	handler.OK(c, role)
}

func (h *Handler) ListPermissions(c *gin.Context) {
	var query handler.PageQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		handler.Fail(c, "INVALID_INPUT", err.Error())
		return
	}

	permissions, pageResult, err := h.authorizationSvc.ListPermissions(c.Request.Context(), svcAuthorization.ListPermissionsCmd{
		Page: query.ToDomain(),
	})
	if err != nil {
		handler.Fail(c, "INTERNAL_ERROR", err.Error())
		return
	}
	handler.OKPage(c, permissions, handler.MetaFromPageResult(pageResult))
}
