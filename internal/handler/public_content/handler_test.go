package public_content

import (
	"context"
	"fmt"
	domainCMS "github.com/freeDog-wy/go-backend-template/internal/domain/cms"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	svcCMS "github.com/freeDog-wy/go-backend-template/internal/usecase/cms"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type contentStub struct {
	result *svcCMS.PublicArticleResult
	err    error
}

func (s contentStub) GetPublishedArticle(context.Context, string, string) (*svcCMS.PublicArticleResult, error) {
	return s.result, s.err
}
func (s contentStub) ListPublishedArticles(context.Context, svcCMS.ListPublicArticlesCmd) ([]*svcCMS.PublicArticleListResult, shared.PageResult, error) {
	return nil, shared.PageResult{}, s.err
}
func (s contentStub) ListPublishedCategoryArticles(context.Context, svcCMS.ListPublicCategoryArticlesCmd) ([]*svcCMS.PublicArticleListResult, shared.PageResult, error) {
	return nil, shared.PageResult{}, s.err
}
func (s contentStub) ListPublishedCategories(context.Context, string) ([]*svcCMS.CategoryTreeResult, error) {
	return nil, s.err
}
func (s contentStub) ListPublicSitemapEntries(context.Context, svcCMS.ListPublicSitemapEntriesCmd) ([]*svcCMS.SitemapEntryResult, shared.PageResult, error) {
	return nil, shared.PageResult{}, s.err
}
func (s contentStub) ResolveRedirect(context.Context, string, string) (*svcCMS.RedirectResult, error) {
	return nil, s.err
}
func (s contentStub) ListPublishedTagArticles(context.Context, svcCMS.ListPublicTagArticlesCmd) ([]*svcCMS.PublicArticleListResult, shared.PageResult, error) {
	return nil, shared.PageResult{}, s.err
}
func TestGetArticleReturnsBusinessNotFoundWithHTTP200(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	New(contentStub{err: fmt.Errorf("wrapped: %w", domainCMS.ErrTranslationAbsent)}).RegisterRoutes(r)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/public/en-US/articles/missing", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "CONTENT_TRANSLATION_NOT_FOUND") {
		t.Fatalf("response = %d %s", w.Code, w.Body.String())
	}
}
