package admin_cms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	svcAuth "github.com/freeDog-wy/go-backend-template/internal/usecase/auth"
	svcAuthorization "github.com/freeDog-wy/go-backend-template/internal/usecase/authorization"
	svcCMS "github.com/freeDog-wy/go-backend-template/internal/usecase/cms"
	"github.com/gin-gonic/gin"
)

func TestSetArticleCoverRequiresPermission(t *testing.T) {
	w := serveCMS(t, false, &cmsServiceFake{}, "/api/v1/admin/cms/articles/7/cover", `{"media_id":12}`)
	assertCMSResponse(t, w, false, "FORBIDDEN")
}

func TestSetArticleCoverRejectsInvalidBody(t *testing.T) {
	w := serveCMS(t, true, &cmsServiceFake{}, "/api/v1/admin/cms/articles/7/cover", `{"media_id":"not-a-number"}`)
	assertCMSResponse(t, w, false, "INVALID_INPUT")
}

func TestSetArticleCoverRejectsInvalidArticleID(t *testing.T) {
	w := serveCMS(t, true, &cmsServiceFake{}, "/api/v1/admin/cms/articles/invalid/cover", `{"media_id":12}`)
	assertCMSResponse(t, w, false, "INVALID_INPUT")
}

func TestSetArticleCoverReturnsSuccess(t *testing.T) {
	service := &cmsServiceFake{}
	w := serveCMS(t, true, service, "/api/v1/admin/cms/articles/7/cover", `{"media_id":12}`)
	assertCMSResponse(t, w, true, "")
	if service.setCover.ArticleID != 7 || service.setCover.ActorUserID != 1 || service.setCover.MediaID == nil || *service.setCover.MediaID != 12 {
		t.Fatalf("set cover command = %#v", service.setCover)
	}
	if !strings.Contains(w.Body.String(), `"id":7`) || !strings.Contains(w.Body.String(), `"cover_media_id":12`) {
		t.Fatalf("response = %s", w.Body.String())
	}
}

func TestSetArticleCoverClearsCover(t *testing.T) {
	service := &cmsServiceFake{}
	w := serveCMS(t, true, service, "/api/v1/admin/cms/articles/7/cover", `{"media_id":null}`)
	assertCMSResponse(t, w, true, "")
	if service.setCover.MediaID != nil || !strings.Contains(w.Body.String(), `"cover_media_id":null`) {
		t.Fatalf("command = %#v, response = %s", service.setCover, w.Body.String())
	}
}

func serveCMS(t *testing.T, allowed bool, service *cmsServiceFake, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	New(&cmsAuthFake{}, &cmsAuthorizerFake{allowed: allowed}, service).RegisterRoutes(r)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func assertCMSResponse(t *testing.T, w *httptest.ResponseRecorder, success bool, code string) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, response = %s", w.Code, w.Body.String())
	}
	if success {
		if !strings.Contains(w.Body.String(), `"success":true`) {
			t.Fatalf("response = %s", w.Body.String())
		}
		return
	}
	if !strings.Contains(w.Body.String(), `"success":false`) || !strings.Contains(w.Body.String(), `"code":"`+code+`"`) {
		t.Fatalf("response = %s", w.Body.String())
	}
}

type cmsAuthFake struct{}

func (*cmsAuthFake) AuthenticateAccessToken(context.Context, string) (*svcAuth.AccessIdentity, error) {
	return &svcAuth.AccessIdentity{UserID: 1}, nil
}

type cmsAuthorizerFake struct{ allowed bool }

func (*cmsAuthorizerFake) EnsureAdminAccess(context.Context, uint) error { return nil }
func (f *cmsAuthorizerFake) HasPermission(context.Context, uint, string) (bool, error) {
	return f.allowed, nil
}

var _ svcAuthorization.AccessAuthorizer = (*cmsAuthorizerFake)(nil)

type cmsServiceFake struct{ setCover svcCMS.SetArticleCoverCmd }

func (*cmsServiceFake) CreateTag(context.Context, svcCMS.CreateTagCmd) (*svcCMS.TagResult, error) {
	return nil, nil
}
func (*cmsServiceFake) UpsertTagTranslation(context.Context, svcCMS.UpsertTagTranslationCmd) (*svcCMS.TagResult, error) {
	return nil, nil
}
func (*cmsServiceFake) ListTags(context.Context, svcCMS.ListTagsCmd) ([]*svcCMS.TagResult, shared.PageResult, error) {
	return nil, shared.PageResult{}, nil
}
func (*cmsServiceFake) ListLocales(context.Context) ([]*svcCMS.LocaleResult, error) { return nil, nil }
func (*cmsServiceFake) CreateLocale(context.Context, svcCMS.CreateLocaleCmd) (*svcCMS.LocaleResult, error) {
	return nil, nil
}
func (*cmsServiceFake) UpdateLocale(context.Context, svcCMS.UpdateLocaleCmd) (*svcCMS.LocaleResult, error) {
	return nil, nil
}
func (*cmsServiceFake) CreateCategory(context.Context, svcCMS.CreateCategoryCmd) (*svcCMS.CategoryResult, error) {
	return nil, nil
}
func (*cmsServiceFake) UpsertCategoryTranslation(context.Context, svcCMS.UpsertCategoryTranslationCmd) (*svcCMS.CategoryResult, error) {
	return nil, nil
}
func (*cmsServiceFake) MoveCategory(context.Context, svcCMS.MoveCategoryCmd) error { return nil }
func (*cmsServiceFake) UpdateCategory(context.Context, svcCMS.UpdateCategoryCmd) (*svcCMS.CategoryResult, error) {
	return nil, nil
}
func (*cmsServiceFake) CreateArticle(context.Context, svcCMS.CreateArticleCmd) (*svcCMS.ArticleResult, error) {
	return nil, nil
}
func (*cmsServiceFake) CreateTranslation(context.Context, svcCMS.CreateTranslationCmd) (*svcCMS.ArticleResult, error) {
	return nil, nil
}
func (*cmsServiceFake) UpdateTranslation(context.Context, svcCMS.UpdateTranslationCmd) (*svcCMS.ArticleResult, error) {
	return nil, nil
}
func (*cmsServiceFake) PublishTranslation(context.Context, svcCMS.PublishTranslationCmd) (*svcCMS.ArticleResult, error) {
	return nil, nil
}
func (*cmsServiceFake) ArchiveTranslation(context.Context, svcCMS.ArchiveTranslationCmd) (*svcCMS.ArticleResult, error) {
	return nil, nil
}
func (*cmsServiceFake) DeleteArticle(context.Context, svcCMS.DeleteArticleCmd) error   { return nil }
func (*cmsServiceFake) RestoreArticle(context.Context, svcCMS.RestoreArticleCmd) error { return nil }
func (f *cmsServiceFake) SetArticleCover(_ context.Context, cmd svcCMS.SetArticleCoverCmd) error {
	f.setCover = cmd
	return nil
}
func (*cmsServiceFake) ListCategories(context.Context, svcCMS.ListCategoriesCmd) ([]*svcCMS.CategoryTreeResult, error) {
	return nil, nil
}
func (*cmsServiceFake) ReplaceArticleCategories(context.Context, svcCMS.ReplaceArticleCategoriesCmd) error {
	return nil
}
func (*cmsServiceFake) ReplaceArticleTags(context.Context, svcCMS.ReplaceArticleTagsCmd) error {
	return nil
}
func (*cmsServiceFake) ListArticles(context.Context, svcCMS.ListArticlesCmd) ([]*svcCMS.ArticleResult, shared.PageResult, error) {
	return nil, shared.PageResult{}, nil
}
func (*cmsServiceFake) GetArticleTranslation(context.Context, svcCMS.GetArticleTranslationCmd) (*svcCMS.ArticleDetailResult, error) {
	return nil, nil
}

var _ svcCMS.AdminService = (*cmsServiceFake)(nil)
