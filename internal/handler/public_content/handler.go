package public_content

import (
	"errors"
	domainCMS "github.com/freeDog-wy/go-backend-template/internal/domain/cms"
	"github.com/freeDog-wy/go-backend-template/internal/handler"
	svcCMS "github.com/freeDog-wy/go-backend-template/internal/usecase/cms"
	"github.com/gin-gonic/gin"
)

type Handler struct{ content svcCMS.PublicContentService }

func New(content svcCMS.PublicContentService) *Handler { return &Handler{content: content} }
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/api/v1/public/:locale/articles/:slug", h.GetArticle)
}
func (h *Handler) GetArticle(c *gin.Context) {
	article, err := h.content.GetPublishedArticle(c, c.Param("locale"), c.Param("slug"))
	if err != nil {
		if errors.Is(err, domainCMS.ErrTranslationAbsent) {
			handler.Fail(c, "CONTENT_TRANSLATION_NOT_FOUND", "published content translation not found")
		} else {
			handler.Fail(c, "INTERNAL_ERROR", err.Error())
		}
		return
	}
	handler.OK(c, article)
}
