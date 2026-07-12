package admin_cms

import (
	"errors"
	domainCMS "github.com/freeDog-wy/go-backend-template/internal/domain/cms"
	"github.com/freeDog-wy/go-backend-template/internal/handler"
	handlerMiddleware "github.com/freeDog-wy/go-backend-template/internal/handler/middleware"
	svcAuth "github.com/freeDog-wy/go-backend-template/internal/usecase/auth"
	svcAuthorization "github.com/freeDog-wy/go-backend-template/internal/usecase/authorization"
	svcCMS "github.com/freeDog-wy/go-backend-template/internal/usecase/cms"
	"github.com/gin-gonic/gin"
	"strconv"
)

type Handler struct {
	auth       svcAuth.AccessAuthenticator
	authorizer svcAuthorization.AccessAuthorizer
	cms        svcCMS.AdminService
}

func New(auth svcAuth.AccessAuthenticator, authorizer svcAuthorization.AccessAuthorizer, cms svcCMS.AdminService) *Handler {
	return &Handler{auth: auth, authorizer: authorizer, cms: cms}
}
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	g := r.Group("/api/v1/admin/cms")
	g.POST("/categories", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.category.manage"), h.CreateCategory)
	g.GET("/categories", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.category.manage"), h.ListCategories)
	g.PATCH("/categories/:id/move", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.category.manage"), h.MoveCategory)
	g.POST("/articles", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.article.create"), h.CreateArticle)
	g.GET("/articles", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.article.update"), h.ListArticles)
	g.PUT("/articles/:id/categories", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.article.update"), h.ReplaceArticleCategories)
	g.POST("/articles/:id/translations", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.article.update"), h.CreateTranslation)
	g.PUT("/articles/:id/translations/:locale", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.article.update"), h.UpdateTranslation)
	g.POST("/articles/:id/translations/:locale/publish", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.article.publish"), h.PublishTranslation)
	g.POST("/articles/:id/translations/:locale/archive", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.article.archive"), h.ArchiveTranslation)
}

type categoryReq struct {
	ParentID       *uint  `json:"parent_id"`
	SortOrder      int    `json:"sort_order"`
	Locale         string `json:"locale" binding:"required"`
	Name           string `json:"name" binding:"required"`
	Slug           string `json:"slug" binding:"required"`
	Description    string `json:"description"`
	SEOTitle       string `json:"seo_title"`
	SEODescription string `json:"seo_description"`
}
type moveReq struct {
	ParentID  *uint `json:"parent_id"`
	SortOrder int   `json:"sort_order"`
}
type replaceArticleCategoriesReq struct {
	CategoryIDs       []uint `json:"category_ids"`
	PrimaryCategoryID *uint  `json:"primary_category_id"`
}
type articleReq struct {
	Locale         string `json:"locale" binding:"required"`
	Title          string `json:"title" binding:"required"`
	Slug           string `json:"slug" binding:"required"`
	Summary        string `json:"summary"`
	Content        string `json:"content"`
	ContentFormat  string `json:"content_format"`
	SEOTitle       string `json:"seo_title"`
	SEODescription string `json:"seo_description"`
	CanonicalURL   string `json:"canonical_url"`
}

func (h *Handler) CreateCategory(c *gin.Context) {
	var req categoryReq
	if c.ShouldBindJSON(&req) != nil {
		invalid(c)
		return
	}
	result, err := h.cms.CreateCategory(c, svcCMS.CreateCategoryCmd{ParentID: req.ParentID, SortOrder: req.SortOrder, Locale: req.Locale, Name: req.Name, Slug: req.Slug, Description: req.Description, SEOTitle: req.SEOTitle, SEODescription: req.SEODescription})
	if err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, result)
}
func (h *Handler) ListCategories(c *gin.Context) {
	locale := c.Query("locale")
	result, err := h.cms.ListCategories(c, svcCMS.ListCategoriesCmd{Locale: locale})
	if err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, result)
}
func (h *Handler) MoveCategory(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req moveReq
	if c.ShouldBindJSON(&req) != nil {
		invalid(c)
		return
	}
	if err := h.cms.MoveCategory(c, svcCMS.MoveCategoryCmd{CategoryID: id, ParentID: req.ParentID, SortOrder: req.SortOrder}); err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, gin.H{"id": id})
}
func (h *Handler) CreateArticle(c *gin.Context) {
	var req articleReq
	if c.ShouldBindJSON(&req) != nil {
		invalid(c)
		return
	}
	result, err := h.cms.CreateArticle(c, svcCMS.CreateArticleCmd{AuthorUserID: handlerMiddleware.CurrentUserID(c), Locale: req.Locale, Title: req.Title, Slug: req.Slug, Summary: req.Summary, Content: req.Content, ContentFormat: req.ContentFormat, SEOTitle: req.SEOTitle, SEODescription: req.SEODescription, CanonicalURL: req.CanonicalURL})
	if err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, result)
}
func (h *Handler) ListArticles(c *gin.Context) {
	var query handler.PageQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		invalid(c)
		return
	}
	results, page, err := h.cms.ListArticles(c, svcCMS.ListArticlesCmd{Locale: c.Query("locale"), Page: query.ToDomain()})
	if err != nil {
		fail(c, err)
		return
	}
	handler.OKPage(c, results, handler.MetaFromPageResult(page))
}
func (h *Handler) ReplaceArticleCategories(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req replaceArticleCategoriesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		invalid(c)
		return
	}
	if err := h.cms.ReplaceArticleCategories(c, svcCMS.ReplaceArticleCategoriesCmd{ArticleID: id, CategoryIDs: req.CategoryIDs, PrimaryCategoryID: req.PrimaryCategoryID}); err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, gin.H{"id": id, "category_ids": req.CategoryIDs, "primary_category_id": req.PrimaryCategoryID})
}
func (h *Handler) CreateTranslation(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req articleReq
	if c.ShouldBindJSON(&req) != nil {
		invalid(c)
		return
	}
	result, err := h.cms.CreateTranslation(c, svcCMS.CreateTranslationCmd{ArticleID: id, Locale: req.Locale, Title: req.Title, Slug: req.Slug, Summary: req.Summary, Content: req.Content, ContentFormat: req.ContentFormat, SEOTitle: req.SEOTitle, SEODescription: req.SEODescription, CanonicalURL: req.CanonicalURL})
	if err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, result)
}
func (h *Handler) UpdateTranslation(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req articleReq
	if c.ShouldBindJSON(&req) != nil {
		invalid(c)
		return
	}
	result, err := h.cms.UpdateTranslation(c, svcCMS.UpdateTranslationCmd{ArticleID: id, Locale: c.Param("locale"), Title: req.Title, Slug: req.Slug, Summary: req.Summary, Content: req.Content, ContentFormat: req.ContentFormat, SEOTitle: req.SEOTitle, SEODescription: req.SEODescription, CanonicalURL: req.CanonicalURL})
	if err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, result)
}
func (h *Handler) PublishTranslation(c *gin.Context) { h.changeState(c, true) }
func (h *Handler) ArchiveTranslation(c *gin.Context) { h.changeState(c, false) }
func (h *Handler) changeState(c *gin.Context, publish bool) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var result *svcCMS.ArticleResult
	var err error
	if publish {
		result, err = h.cms.PublishTranslation(c, svcCMS.PublishTranslationCmd{ArticleID: id, Locale: c.Param("locale")})
	} else {
		result, err = h.cms.ArchiveTranslation(c, svcCMS.ArchiveTranslationCmd{ArticleID: id, Locale: c.Param("locale")})
	}
	if err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, result)
}
func idParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		handler.Fail(c, "INVALID_INPUT", "invalid resource ID")
		return 0, false
	}
	return uint(id), true
}
func invalid(c *gin.Context) { handler.Fail(c, "INVALID_INPUT", "invalid request") }
func fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domainCMS.ErrInvalidInput):
		handler.Fail(c, "INVALID_INPUT", "invalid CMS input")
	case errors.Is(err, domainCMS.ErrLocaleNotFound):
		handler.Fail(c, "LOCALE_NOT_FOUND", "locale is not enabled")
	case errors.Is(err, domainCMS.ErrCategoryNotFound):
		handler.Fail(c, "CATEGORY_NOT_FOUND", "category not found")
	case errors.Is(err, domainCMS.ErrCategoryCycle):
		handler.Fail(c, "CATEGORY_CYCLE", "category hierarchy cannot contain a cycle")
	case errors.Is(err, domainCMS.ErrTranslationAbsent):
		handler.Fail(c, "CONTENT_TRANSLATION_NOT_FOUND", "content translation not found")
	case errors.Is(err, domainCMS.ErrArticleNotFound):
		handler.Fail(c, "ARTICLE_NOT_FOUND", "article not found")
	default:
		handler.Fail(c, "INTERNAL_ERROR", err.Error())
	}
}
