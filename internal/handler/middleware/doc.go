// Package middleware 提供 HTTP 请求的横切处理：认证、授权、限流、幂等和恢复。
//
// 中间件在 Handler 之前建立协议和安全边界，不承载业务流程。写接口的幂等中间件会
// 保存并重放完整 HTTP 响应，因此只应装配到明确支持 Idempotency-Key 的路由。
package middleware
