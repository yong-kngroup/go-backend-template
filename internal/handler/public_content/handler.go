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
	r.GET("/api/v1/public/:locale/categories", h.ListCategories)
	r.GET("/api/v1/public/:locale/articles", h.ListArticles)
	r.GET("/api/v1/public/:locale/categories/:slug/articles", h.ListCategoryArticles)
	r.GET("/api/v1/public/:locale/articles/:slug", h.GetArticle)
}
func (h *Handler) ListCategories(c *gin.Context) {
	result, err := h.content.ListPublishedCategories(c, c.Param("locale"))
	if err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, result)
}
func (h *Handler) ListArticles(c *gin.Context) {
	var query handler.PageQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		handler.Fail(c, "INVALID_INPUT", "invalid page query")
		return
	}
	results, page, err := h.content.ListPublishedArticles(c, svcCMS.ListPublicArticlesCmd{Locale: c.Param("locale"), Page: query.ToDomain()})
	if err != nil {
		fail(c, err)
		return
	}
	handler.OKPage(c, results, handler.MetaFromPageResult(page))
}
func (h *Handler) ListCategoryArticles(c *gin.Context) {
	var query handler.PageQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		handler.Fail(c, "INVALID_INPUT", "invalid page query")
		return
	}
	results, page, err := h.content.ListPublishedCategoryArticles(c, svcCMS.ListPublicCategoryArticlesCmd{Locale: c.Param("locale"), CategorySlug: c.Param("slug"), Page: query.ToDomain()})
	if err != nil {
		fail(c, err)
		return
	}
	handler.OKPage(c, results, handler.MetaFromPageResult(page))
}
func (h *Handler) GetArticle(c *gin.Context) {
	article, err := h.content.GetPublishedArticle(c, c.Param("locale"), c.Param("slug"))
	if err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, article)
}
func fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domainCMS.ErrTranslationAbsent):
		handler.Fail(c, "CONTENT_TRANSLATION_NOT_FOUND", "published content translation not found")
	case errors.Is(err, domainCMS.ErrCategoryNotFound):
		handler.Fail(c, "CATEGORY_NOT_FOUND", "published category not found")
	case errors.Is(err, domainCMS.ErrLocaleNotFound):
		handler.Fail(c, "LOCALE_NOT_FOUND", "locale is not enabled")
	case errors.Is(err, domainCMS.ErrInvalidInput):
		handler.Fail(c, "INVALID_INPUT", "invalid public content query")
	default:
		handler.Fail(c, "INTERNAL_ERROR", err.Error())
	}
}
