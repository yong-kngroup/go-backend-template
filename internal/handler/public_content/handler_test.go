package public_content

import (
	"context"
	"fmt"
	domainCMS "github.com/freeDog-wy/go-backend-template/internal/domain/cms"
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
