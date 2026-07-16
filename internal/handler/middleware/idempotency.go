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

// IdempotencyKeyHeader 是客户端声明写请求幂等性的 HTTP 请求头。
const IdempotencyKeyHeader = "Idempotency-Key"

// IdempotencyRecord 表示某个幂等键已领取的请求及其可重放响应。
type IdempotencyRecord struct {
	ID           uint
	RequestHash  string
	ResponseBody []byte
	StatusCode   int
	CompletedAt  *time.Time
}

// IdempotencyStore 原子地领取幂等键并保存首次请求的响应。
//
// 幂等命名空间由 actor、method、route 和 key 组成；requestHash 用于拒绝同一 key
// 对应不同请求体的情况。Claim 返回 claimed=false 时，调用方必须根据记录状态重放、
// 拒绝或报告处理中，而不能再次执行业务逻辑。
type IdempotencyStore interface {
	// Claim 返回已创建的记录和 true，或返回既有记录和 false。
	Claim(context.Context, uint, string, string, string, string) (*IdempotencyRecord, bool, error)
	// Complete 只完成尚未完成的记录，保存原始响应以供同一请求重放。
	Complete(context.Context, uint, []byte, int) error
}

// Idempotency 为携带 Idempotency-Key 的写请求提供重放保护。
//
// 无 key 的请求保持原有行为。已领取但未完成的记录不会执行第二次；首次业务响应已经
// 写出后，若 Complete 失败也不能用基础设施错误替换该响应，以避免客户端误判业务失败。
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
