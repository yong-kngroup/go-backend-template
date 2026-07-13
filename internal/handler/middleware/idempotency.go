package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/handler"
	"github.com/gin-gonic/gin"
)

const IdempotencyKeyHeader = "Idempotency-Key"

type IdempotencyRecord struct {
	ID           uint
	RequestHash  string
	ResponseBody []byte
	StatusCode   int
	CompletedAt  *time.Time
}

type IdempotencyStore interface {
	Claim(context.Context, uint, string, string, string, string) (*IdempotencyRecord, bool, error)
	Complete(context.Context, uint, []byte, int) error
}

// Idempotency replays a completed response for an identical write request.
func Idempotency(store IdempotencyStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := strings.TrimSpace(c.GetHeader(IdempotencyKeyHeader))
		if key == "" {
			c.Next()
			return
		}
		if len(key) > 200 {
			handler.Fail(c, "INVALID_IDEMPOTENCY_KEY", "idempotency key is invalid")
			c.Abort()
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			handler.Fail(c, "INVALID_INPUT", "request body cannot be read")
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		hash := sha256.Sum256(body)
		record, claimed, err := store.Claim(c.Request.Context(), CurrentUserID(c), c.Request.Method, c.FullPath(), key, hex.EncodeToString(hash[:]))
		if err != nil {
			handler.Fail(c, "IDEMPOTENCY_UNAVAILABLE", "idempotency check failed")
			c.Abort()
			return
		}
		if !claimed {
			if record.RequestHash != hex.EncodeToString(hash[:]) {
				handler.Fail(c, "IDEMPOTENCY_KEY_REUSED", "idempotency key was used with a different request")
			} else if record.CompletedAt == nil {
				handler.Fail(c, "IDEMPOTENCY_IN_PROGRESS", "an identical request is still being processed")
			} else {
				c.Data(record.StatusCode, "application/json; charset=utf-8", record.ResponseBody)
			}
			c.Abort()
			return
		}

		writer := &idempotencyWriter{ResponseWriter: c.Writer}
		c.Writer = writer
		c.Next()
		if err := store.Complete(c.Request.Context(), record.ID, writer.body.Bytes(), c.Writer.Status()); err != nil {
			// The operation succeeded; do not replace its response with an infrastructure error.
			return
		}
	}
}

type idempotencyWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *idempotencyWriter) Write(data []byte) (int, error) {
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *idempotencyWriter) WriteString(value string) (int, error) {
	w.body.WriteString(value)
	return w.ResponseWriter.WriteString(value)
}

var _ http.ResponseWriter = (*idempotencyWriter)(nil)
