package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestIdempotencyReplaysCompletedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &idempotencyStoreFake{}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(CurrentUserIDKey, uint(7)); c.Next() })
	r.POST("/writes", Idempotency(store), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"success": true}) })

	first := requestWithKey(r, "same-key", `{"title":"one"}`)
	second := requestWithKey(r, "same-key", `{"title":"one"}`)
	if first.Body.String() != second.Body.String() || store.claims != 2 || store.completes != 1 {
		t.Fatalf("first=%s second=%s claims=%d completes=%d", first.Body.String(), second.Body.String(), store.claims, store.completes)
	}
}

func TestIdempotencyRejectsKeyReusedWithDifferentBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &idempotencyStoreFake{}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(CurrentUserIDKey, uint(7)); c.Next() })
	r.POST("/writes", Idempotency(store), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"success": true}) })
	_ = requestWithKey(r, "same-key", `{"title":"one"}`)
	response := requestWithKey(r, "same-key", `{"title":"two"}`)
	if !strings.Contains(response.Body.String(), `"IDEMPOTENCY_KEY_REUSED"`) || store.completes != 1 {
		t.Fatalf("response=%s completes=%d", response.Body.String(), store.completes)
	}
}

func requestWithKey(r http.Handler, key, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/writes", strings.NewReader(body))
	req.Header.Set(IdempotencyKeyHeader, key)
	r.ServeHTTP(w, req)
	return w
}

type idempotencyStoreFake struct {
	record    *IdempotencyRecord
	claims    int
	completes int
}

func (s *idempotencyStoreFake) Claim(_ context.Context, _ uint, _ string, _ string, _ string, requestHash string) (*IdempotencyRecord, bool, error) {
	s.claims++
	if s.record == nil {
		s.record = &IdempotencyRecord{ID: 1, RequestHash: requestHash}
		return s.record, true, nil
	}
	return s.record, false, nil
}
func (s *idempotencyStoreFake) Complete(_ context.Context, _ uint, body []byte, status int) error {
	s.completes++
	now := time.Now()
	s.record.ResponseBody, s.record.StatusCode, s.record.CompletedAt = body, status, &now
	return nil
}
