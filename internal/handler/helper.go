package handler

import (
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"

	"github.com/gin-gonic/gin"
)

// Response 标准 API 响应信封。
type Response struct {
	Success bool       `json:"success"`
	Data    any        `json:"data,omitempty"`
	Error   *ErrorInfo `json:"error,omitempty"`
	Meta    *Meta      `json:"meta,omitempty"`
}

// ErrorInfo 错误详情。
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Meta 分页元数据，非分页接口省略。
type Meta struct {
	Page       int   `json:"page,omitempty"`
	PerPage    int   `json:"per_page,omitempty"`
	Total      int64 `json:"total,omitempty"`
	TotalPages int   `json:"total_pages,omitempty"`
}

type PageQuery struct {
	Page    int `form:"page"`
	PerPage int `form:"per_page"`
}

type RequestAuditMeta struct {
	IP            string
	UserAgent     string
	CorrelationID string
}

// OK 返回成功响应（200）。
func OK(c *gin.Context, data any) {
	c.JSON(200, Response{
		Success: true,
		Data:    data,
	})
}

// OKPage 返回成功响应带分页元数据。
func OKPage(c *gin.Context, data any, meta *Meta) {
	c.JSON(200, Response{
		Success: true,
		Data:    data,
		Meta:    meta,
	})
}

func (q PageQuery) ToDomain() shared.PageQuery {
	return shared.NewPageQuery(q.Page, q.PerPage)
}

func MetaFromPageResult(result shared.PageResult) *Meta {
	return &Meta{
		Page:       result.Page,
		PerPage:    result.PerPage,
		Total:      result.Total,
		TotalPages: result.TotalPages(),
	}
}

func AuditMetaFromRequest(c *gin.Context) RequestAuditMeta {
	return RequestAuditMeta{
		IP:            c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
		CorrelationID: c.GetHeader("X-Correlation-ID"),
	}
}

// Fail 返回业务错误响应。
func Fail(c *gin.Context, code, message string) {
	c.JSON(200, Response{
		Success: false,
		Error:   &ErrorInfo{Code: code, Message: message},
	})
}
