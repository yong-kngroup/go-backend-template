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
	auth        svcAuth.AccessAuthenticator
	authorizer  svcAuthorization.AccessAuthorizer
	cms         svcCMS.AdminService
	idempotency gin.HandlerFunc
}

func New(auth svcAuth.AccessAuthenticator, authorizer svcAuthorization.AccessAuthorizer, cms svcCMS.AdminService) *Handler {
	return &Handler{auth: auth, authorizer: authorizer, cms: cms}
}
func (h *Handler) SetIdempotency(mw gin.HandlerFunc) { h.idempotency = mw }
func (h *Handler) writeHandlers(permission string, endpoint gin.HandlerFunc) []gin.HandlerFunc {
	handlers := []gin.HandlerFunc{handlerMiddleware.RequirePermission(h.auth, h.authorizer, permission)}
	if h.idempotency != nil {
		handlers = append(handlers, h.idempotency)
	}
	return append(handlers, endpoint)
}
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	g := r.Group("/api/v1/admin/cms")
	g.GET("/locales", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.locale.manage"), h.ListLocales)
	g.POST("/locales", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.locale.manage"), h.CreateLocale)
	g.GET("/tags", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.tag.manage"), h.ListTags)
	g.POST("/tags", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.tag.manage"), h.CreateTag)
	g.PUT("/tags/:id/translations/:locale", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.tag.manage"), h.UpsertTagTranslation)
	g.PATCH("/locales/:code", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.locale.manage"), h.UpdateLocale)
	g.POST("/categories", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.category.manage"), h.CreateCategory)
	g.GET("/categories", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.category.manage"), h.ListCategories)
	g.PATCH("/categories/:id/move", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.category.manage"), h.MoveCategory)
	g.PATCH("/categories/:id", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.category.manage"), h.UpdateCategory)
	g.PUT("/categories/:id/translations/:locale", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.category.manage"), h.UpsertCategoryTranslation)
	g.POST("/articles", h.writeHandlers("cms.article.create", h.CreateArticle)...)
	g.GET("/articles", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.article.update"), h.ListArticles)
	g.DELETE("/articles/:id", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.article.archive"), h.DeleteArticle)
	g.POST("/articles/:id/restore", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.article.archive"), h.RestoreArticle)
	g.GET("/articles/:id/translations/:locale", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.article.update"), h.GetArticleTranslation)
	g.PUT("/articles/:id/categories", h.writeHandlers("cms.article.update", h.ReplaceArticleCategories)...)
	g.PUT("/articles/:id/tags", h.writeHandlers("cms.article.update", h.ReplaceArticleTags)...)
	g.PUT("/articles/:id/cover", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.article.update"), h.SetArticleCover)
	g.POST("/articles/:id/translations", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.article.update"), h.CreateTranslation)
	g.PUT("/articles/:id/translations/:locale", h.writeHandlers("cms.article.update", h.UpdateTranslation)...)
	g.GET("/articles/:id/translations/:locale/publish-preview", handlerMiddleware.RequirePermission(h.auth, h.authorizer, "cms.article.publish"), h.PreviewPublish)
	g.POST("/articles/:id/translations/:locale/publish", h.writeHandlers("cms.article.publish", h.PublishTranslation)...)
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
type updateCategoryReq struct {
	IsEnabled bool `json:"is_enabled"`
	SortOrder int  `json:"sort_order"`
}
type replaceArticleCategoriesReq struct {
	CategoryIDs       []uint `json:"category_ids"`
	PrimaryCategoryID *uint  `json:"primary_category_id"`
}
type localeReq struct {
	Code      string `json:"code" binding:"required"`
	Name      string `json:"name" binding:"required"`
	IsEnabled bool   `json:"is_enabled"`
	SortOrder int    `json:"sort_order"`
	IsDefault bool   `json:"is_default"`
}
type updateLocaleReq struct {
	Name      string `json:"name" binding:"required"`
	IsEnabled bool   `json:"is_enabled"`
	SortOrder int    `json:"sort_order"`
	IsDefault bool   `json:"is_default"`
}
type categoryTranslationReq struct {
	Name           string `json:"name" binding:"required"`
	Slug           string `json:"slug" binding:"required"`
	Description    string `json:"description"`
	SEOTitle       string `json:"seo_title"`
	SEODescription string `json:"seo_description"`
}
type tagReq struct {
	Locale string `json:"locale" binding:"required"`
	Name   string `json:"name" binding:"required"`
	Slug   string `json:"slug" binding:"required"`
}
type tagTranslationReq struct {
	Name string `json:"name" binding:"required"`
	Slug string `json:"slug" binding:"required"`
}
type replaceTagsReq struct {
	TagIDs []uint `json:"tag_ids"`
}
type articleCoverReq struct {
	MediaID *uint `json:"media_id"`
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
func (h *Handler) ListLocales(c *gin.Context) {
	result, err := h.cms.ListLocales(c)
	if err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, result)
}
func (h *Handler) CreateLocale(c *gin.Context) {
	var req localeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		invalid(c)
		return
	}
	meta := handler.AuditMetaFromRequest(c)
	result, err := h.cms.CreateLocale(c, svcCMS.CreateLocaleCmd{Code: req.Code, Name: req.Name, SortOrder: req.SortOrder, ActorUserID: handlerMiddleware.CurrentUserID(c), IP: meta.IP, UserAgent: meta.UserAgent})
	if err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, result)
}
func (h *Handler) ListTags(c *gin.Context) {
	var q handler.PageQuery
	if c.ShouldBindQuery(&q) != nil {
		invalid(c)
		return
	}
	r, p, e := h.cms.ListTags(c, svcCMS.ListTagsCmd{Locale: c.Query("locale"), Page: q.ToDomain()})
	if e != nil {
		fail(c, e)
		return
	}
	handler.OKPage(c, r, handler.MetaFromPageResult(p))
}
func (h *Handler) CreateTag(c *gin.Context) {
	var req tagReq
	if c.ShouldBindJSON(&req) != nil {
		invalid(c)
		return
	}
	m := handler.AuditMetaFromRequest(c)
	r, e := h.cms.CreateTag(c, svcCMS.CreateTagCmd{Locale: req.Locale, Name: req.Name, Slug: req.Slug, ActorUserID: handlerMiddleware.CurrentUserID(c), IP: m.IP, UserAgent: m.UserAgent})
	if e != nil {
		fail(c, e)
		return
	}
	handler.OK(c, r)
}
func (h *Handler) UpsertTagTranslation(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req tagTranslationReq
	if c.ShouldBindJSON(&req) != nil {
		invalid(c)
		return
	}
	m := handler.AuditMetaFromRequest(c)
	r, e := h.cms.UpsertTagTranslation(c, svcCMS.UpsertTagTranslationCmd{TagID: id, Locale: c.Param("locale"), Name: req.Name, Slug: req.Slug, ActorUserID: handlerMiddleware.CurrentUserID(c), IP: m.IP, UserAgent: m.UserAgent})
	if e != nil {
		fail(c, e)
		return
	}
	handler.OK(c, r)
}
func (h *Handler) UpdateLocale(c *gin.Context) {
	var req updateLocaleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		invalid(c)
		return
	}
	meta := handler.AuditMetaFromRequest(c)
	result, err := h.cms.UpdateLocale(c, svcCMS.UpdateLocaleCmd{Code: c.Param("code"), Name: req.Name, IsEnabled: req.IsEnabled, SortOrder: req.SortOrder, IsDefault: req.IsDefault, ActorUserID: handlerMiddleware.CurrentUserID(c), IP: meta.IP, UserAgent: meta.UserAgent})
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
	meta := handler.AuditMetaFromRequest(c)
	if err := h.cms.MoveCategory(c, svcCMS.MoveCategoryCmd{CategoryID: id, ParentID: req.ParentID, SortOrder: req.SortOrder, ActorUserID: handlerMiddleware.CurrentUserID(c), IP: meta.IP, UserAgent: meta.UserAgent}); err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, gin.H{"id": id})
}
func (h *Handler) UpdateCategory(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req updateCategoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		invalid(c)
		return
	}
	meta := handler.AuditMetaFromRequest(c)
	result, err := h.cms.UpdateCategory(c, svcCMS.UpdateCategoryCmd{CategoryID: id, IsEnabled: req.IsEnabled, SortOrder: req.SortOrder, ActorUserID: handlerMiddleware.CurrentUserID(c), IP: meta.IP, UserAgent: meta.UserAgent})
	if err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, result)
}
func (h *Handler) UpsertCategoryTranslation(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req categoryTranslationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		invalid(c)
		return
	}
	meta := handler.AuditMetaFromRequest(c)
	result, err := h.cms.UpsertCategoryTranslation(c, svcCMS.UpsertCategoryTranslationCmd{CategoryID: id, Locale: c.Param("locale"), Name: req.Name, Slug: req.Slug, Description: req.Description, SEOTitle: req.SEOTitle, SEODescription: req.SEODescription, ActorUserID: handlerMiddleware.CurrentUserID(c), IP: meta.IP, UserAgent: meta.UserAgent})
	if err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, result)
}
func (h *Handler) CreateArticle(c *gin.Context) {
	var req articleReq
	if c.ShouldBindJSON(&req) != nil {
		invalid(c)
		return
	}
	meta := handler.AuditMetaFromRequest(c)
	result, err := h.cms.CreateArticle(c, svcCMS.CreateArticleCmd{AuthorUserID: handlerMiddleware.CurrentUserID(c), Locale: req.Locale, Title: req.Title, Slug: req.Slug, Summary: req.Summary, Content: req.Content, ContentFormat: req.ContentFormat, SEOTitle: req.SEOTitle, SEODescription: req.SEODescription, CanonicalURL: req.CanonicalURL, IP: meta.IP, UserAgent: meta.UserAgent, CorrelationID: meta.CorrelationID})
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
	results, page, err := h.cms.ListArticles(c, svcCMS.ListArticlesCmd{Locale: c.Query("locale"), IncludeDeleted: c.Query("include_deleted") == "true", Page: query.ToDomain()})
	if err != nil {
		fail(c, err)
		return
	}
	handler.OKPage(c, results, handler.MetaFromPageResult(page))
}
func (h *Handler) DeleteArticle(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	meta := handler.AuditMetaFromRequest(c)
	if err := h.cms.DeleteArticle(c, svcCMS.DeleteArticleCmd{ArticleID: id, ActorUserID: handlerMiddleware.CurrentUserID(c), IP: meta.IP, UserAgent: meta.UserAgent}); err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, gin.H{"id": id})
}
func (h *Handler) RestoreArticle(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	meta := handler.AuditMetaFromRequest(c)
	if err := h.cms.RestoreArticle(c, svcCMS.RestoreArticleCmd{ArticleID: id, ActorUserID: handlerMiddleware.CurrentUserID(c), IP: meta.IP, UserAgent: meta.UserAgent}); err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, gin.H{"id": id})
}
func (h *Handler) GetArticleTranslation(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	result, err := h.cms.GetArticleTranslation(c, svcCMS.GetArticleTranslationCmd{ArticleID: id, Locale: c.Param("locale")})
	if err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, result)
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
	m := handler.AuditMetaFromRequest(c)
	if err := h.cms.ReplaceArticleCategories(c, svcCMS.ReplaceArticleCategoriesCmd{ArticleID: id, CategoryIDs: req.CategoryIDs, PrimaryCategoryID: req.PrimaryCategoryID, ActorUserID: handlerMiddleware.CurrentUserID(c), IP: m.IP, UserAgent: m.UserAgent, CorrelationID: m.CorrelationID}); err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, gin.H{"id": id, "category_ids": req.CategoryIDs, "primary_category_id": req.PrimaryCategoryID})
}
func (h *Handler) ReplaceArticleTags(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req replaceTagsReq
	if c.ShouldBindJSON(&req) != nil {
		invalid(c)
		return
	}
	m := handler.AuditMetaFromRequest(c)
	if e := h.cms.ReplaceArticleTags(c, svcCMS.ReplaceArticleTagsCmd{ArticleID: id, TagIDs: req.TagIDs, ActorUserID: handlerMiddleware.CurrentUserID(c), IP: m.IP, UserAgent: m.UserAgent, CorrelationID: m.CorrelationID}); e != nil {
		fail(c, e)
		return
	}
	handler.OK(c, gin.H{"id": id, "tag_ids": req.TagIDs})
}
func (h *Handler) SetArticleCover(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req articleCoverReq
	if c.ShouldBindJSON(&req) != nil {
		invalid(c)
		return
	}
	meta := handler.AuditMetaFromRequest(c)
	if err := h.cms.SetArticleCover(c, svcCMS.SetArticleCoverCmd{ArticleID: id, MediaID: req.MediaID, ActorUserID: handlerMiddleware.CurrentUserID(c), IP: meta.IP, UserAgent: meta.UserAgent}); err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, gin.H{"id": id, "cover_media_id": req.MediaID})
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
	meta := handler.AuditMetaFromRequest(c)
	result, err := h.cms.UpdateTranslation(c, svcCMS.UpdateTranslationCmd{ArticleID: id, Locale: c.Param("locale"), Title: req.Title, Slug: req.Slug, Summary: req.Summary, Content: req.Content, ContentFormat: req.ContentFormat, SEOTitle: req.SEOTitle, SEODescription: req.SEODescription, CanonicalURL: req.CanonicalURL, ActorUserID: handlerMiddleware.CurrentUserID(c), IP: meta.IP, UserAgent: meta.UserAgent, CorrelationID: meta.CorrelationID})
	if err != nil {
		fail(c, err)
		return
	}
	handler.OK(c, result)
}
func (h *Handler) PreviewPublish(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	result, err := h.cms.PreviewPublish(c, svcCMS.PreviewPublishCmd{ArticleID: id, Locale: c.Param("locale")})
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
		meta := handler.AuditMetaFromRequest(c)
		result, err = h.cms.PublishTranslation(c, svcCMS.PublishTranslationCmd{ArticleID: id, Locale: c.Param("locale"), ActorUserID: handlerMiddleware.CurrentUserID(c), IP: meta.IP, UserAgent: meta.UserAgent, CorrelationID: meta.CorrelationID})
	} else {
		meta := handler.AuditMetaFromRequest(c)
		result, err = h.cms.ArchiveTranslation(c, svcCMS.ArchiveTranslationCmd{ArticleID: id, Locale: c.Param("locale"), ActorUserID: handlerMiddleware.CurrentUserID(c), IP: meta.IP, UserAgent: meta.UserAgent})
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
	case errors.Is(err, domainCMS.ErrLocaleDefault):
		handler.Fail(c, "DEFAULT_LOCALE_CANNOT_BE_DISABLED", "default locale cannot be disabled")
	case errors.Is(err, domainCMS.ErrLastEnabledLocale):
		handler.Fail(c, "LAST_ENABLED_LOCALE", "at least one locale must remain enabled")
	case errors.Is(err, domainCMS.ErrArticleDeleted):
		handler.Fail(c, "ARTICLE_ALREADY_DELETED", "article is already deleted")
	case errors.Is(err, domainCMS.ErrArticleActive):
		handler.Fail(c, "ARTICLE_NOT_DELETED", "article is not deleted")
	case errors.Is(err, domainCMS.ErrSlugReserved):
		handler.Fail(c, "SLUG_RESERVED", "slug is reserved by a redirect")
	case errors.Is(err, domainCMS.ErrTagNotFound):
		handler.Fail(c, "TAG_NOT_FOUND", "tag not found")
	case errors.Is(err, domainCMS.ErrPublicationNotReady):
		handler.Fail(c, "CONTENT_NOT_READY_FOR_PUBLICATION", "content did not pass publication checks")
	default:
		handler.Fail(c, "INTERNAL_ERROR", err.Error())
	}
}
