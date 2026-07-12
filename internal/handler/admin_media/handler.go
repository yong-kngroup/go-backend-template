package admin_media

import (
	"errors"
	"fmt"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	"github.com/freeDog-wy/go-backend-template/internal/handler"
	mw "github.com/freeDog-wy/go-backend-template/internal/handler/middleware"
	auth "github.com/freeDog-wy/go-backend-template/internal/usecase/auth"
	authorization "github.com/freeDog-wy/go-backend-template/internal/usecase/authorization"
	media "github.com/freeDog-wy/go-backend-template/internal/usecase/media"
	"github.com/gin-gonic/gin"
	"strings"
)

type Handler struct {
	auth       auth.AccessAuthenticator
	authorizer authorization.AccessAuthorizer
	media      media.MediaAdminService
}

func New(a auth.AccessAuthenticator, z authorization.AccessAuthorizer, m media.MediaAdminService) *Handler {
	return &Handler{auth: a, authorizer: z, media: m}
}
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	g := r.Group("/api/v1/admin/cms/media")
	g.POST("/upload-requests", mw.RequirePermission(h.auth, h.authorizer, "cms.media.upload"), h.RequestUpload)
	g.POST("/:id/complete", mw.RequirePermission(h.auth, h.authorizer, "cms.media.upload"), h.Complete)
	g.GET("", mw.RequirePermission(h.auth, h.authorizer, "cms.media.upload"), h.List)
	g.PUT("/:id/translations/:locale", mw.RequirePermission(h.auth, h.authorizer, "cms.media.upload"), h.Translate)
}

type uploadReq struct {
	Filename    string `json:"filename" binding:"required"`
	ContentType string `json:"content_type" binding:"required"`
	SizeBytes   int64  `json:"size_bytes" binding:"required"`
}

func (h *Handler) RequestUpload(c *gin.Context) {
	var req uploadReq
	if c.ShouldBindJSON(&req) != nil {
		handler.Fail(c, "INVALID_INPUT", "invalid media upload request")
		return
	}
	out, err := h.media.RequestUpload(c, media.UploadRequest{Filename: req.Filename, ContentType: req.ContentType, SizeBytes: req.SizeBytes, UserID: mw.CurrentUserID(c)})
	if err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, out)
}
func (h *Handler) Complete(c *gin.Context) {
	id := c.Param("id")
	var n uint
	_, err := fmt.Sscan(id, &n)
	if err != nil || n == 0 {
		handler.Fail(c, "INVALID_INPUT", "invalid media ID")
		return
	}
	if err = h.media.Complete(c, n, mw.CurrentUserID(c)); err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, gin.H{"id": n, "status": "ready"})
}
func (h *Handler) List(c *gin.Context) {
	page := 1
	per := 20
	fmt.Sscan(c.Query("page"), &page)
	fmt.Sscan(c.Query("per_page"), &per)
	items, total, err := h.media.List(c, page, per)
	if err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, gin.H{"items": items, "total": total, "page": page, "per_page": per})
}
func (h *Handler) Translate(c *gin.Context) {
	var n uint
	if _, err := fmt.Sscan(c.Param("id"), &n); err != nil || n == 0 {
		handler.Fail(c, "INVALID_INPUT", "invalid media ID")
		return
	}
	var req struct {
		AltText string `json:"alt_text"`
		Title   string `json:"title"`
	}
	if c.ShouldBindJSON(&req) != nil {
		handler.Fail(c, "INVALID_INPUT", "invalid media translation")
		return
	}
	if err := h.media.UpsertTranslation(c, n, c.Param("locale"), req.AltText, req.Title); err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, gin.H{"id": n, "locale": c.Param("locale")})
}
func fail(c *gin.Context, err error) {
	if errors.Is(err, media.ErrMediaValidationFailed) {
		handler.Fail(c, "MEDIA_VALIDATION_FAILED", "uploaded object is not a valid image")
		return
	}
	if errors.Is(err, shared.ErrNotFound) {
		handler.Fail(c, "MEDIA_NOT_FOUND", "media not found")
		return
	}
	if strings.Contains(err.Error(), "not configured") {
		handler.Fail(c, "MEDIA_STORAGE_NOT_CONFIGURED", "object storage is not configured")
		return
	}
	handler.Fail(c, "INVALID_INPUT", err.Error())
}
