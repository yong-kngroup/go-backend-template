package admin_media

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	svcAuth "github.com/freeDog-wy/go-backend-template/internal/usecase/auth"
	svcAuthorization "github.com/freeDog-wy/go-backend-template/internal/usecase/authorization"
	svcMedia "github.com/freeDog-wy/go-backend-template/internal/usecase/media"
	"github.com/gin-gonic/gin"
)

func TestMediaRoutesRequirePermission(t *testing.T) {
	w := serveMedia(t, false, &mediaServiceFake{}, http.MethodPost, "/api/v1/admin/cms/media/upload-requests", `{"filename":"cover.png","content_type":"image/png","size_bytes":12}`)
	assertMediaResponse(t, w, false, "FORBIDDEN")
}

func TestRequestUploadRejectsInvalidBody(t *testing.T) {
	w := serveMedia(t, true, &mediaServiceFake{}, http.MethodPost, "/api/v1/admin/cms/media/upload-requests", `{"filename":"cover.png"}`)
	assertMediaResponse(t, w, false, "INVALID_INPUT")
}

func TestCompleteMapsValidationFailure(t *testing.T) {
	w := serveMedia(t, true, &mediaServiceFake{completeErr: svcMedia.ErrMediaValidationFailed}, http.MethodPost, "/api/v1/admin/cms/media/12/complete", "")
	assertMediaResponse(t, w, false, "MEDIA_VALIDATION_FAILED")
}

func TestCompleteMapsExpiredUpload(t *testing.T) {
	w := serveMedia(t, true, &mediaServiceFake{completeErr: svcMedia.ErrMediaUploadExpired}, http.MethodPost, "/api/v1/admin/cms/media/12/complete", "")
	assertMediaResponse(t, w, false, "MEDIA_UPLOAD_EXPIRED")
}

func TestRequestUploadReturnsSuccess(t *testing.T) {
	service := &mediaServiceFake{uploadResult: &svcMedia.UploadResult{ID: 12, UploadURL: "https://storage.example/upload"}}
	w := serveMedia(t, true, service, http.MethodPost, "/api/v1/admin/cms/media/upload-requests", `{"filename":"cover.png","content_type":"image/png","size_bytes":12}`)
	assertMediaResponse(t, w, true, "")
	if service.uploadRequest.UserID != 1 || service.uploadRequest.Filename != "cover.png" {
		t.Fatalf("upload request = %#v", service.uploadRequest)
	}
	if !strings.Contains(w.Body.String(), `"id":12`) {
		t.Fatalf("response = %s", w.Body.String())
	}
}

func serveMedia(t *testing.T, allowed bool, service *mediaServiceFake, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	New(&mediaAuthFake{}, &mediaAuthorizerFake{allowed: allowed}, service).RegisterRoutes(r)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func assertMediaResponse(t *testing.T, w *httptest.ResponseRecorder, success bool, code string) {
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

type mediaAuthFake struct{}

func (*mediaAuthFake) AuthenticateAccessToken(context.Context, string) (*svcAuth.AccessIdentity, error) {
	return &svcAuth.AccessIdentity{UserID: 1}, nil
}

type mediaAuthorizerFake struct{ allowed bool }

func (*mediaAuthorizerFake) EnsureAdminAccess(context.Context, uint) error { return nil }
func (f *mediaAuthorizerFake) HasPermission(context.Context, uint, string) (bool, error) {
	return f.allowed, nil
}

var _ svcAuthorization.AccessAuthorizer = (*mediaAuthorizerFake)(nil)

type mediaServiceFake struct {
	uploadRequest svcMedia.UploadRequest
	uploadResult  *svcMedia.UploadResult
	uploadErr     error
	completeErr   error
}

func (f *mediaServiceFake) RequestUpload(_ context.Context, request svcMedia.UploadRequest) (*svcMedia.UploadResult, error) {
	f.uploadRequest = request
	return f.uploadResult, f.uploadErr
}
func (f *mediaServiceFake) Complete(context.Context, uint, uint) error { return f.completeErr }
func (*mediaServiceFake) List(context.Context, int, int) ([]svcMedia.MediaResult, int64, error) {
	return nil, 0, nil
}
func (*mediaServiceFake) UpsertTranslation(context.Context, uint, string, string, string) error {
	return nil
}

var (
	_ svcMedia.MediaAdminService = (*mediaServiceFake)(nil)
)
