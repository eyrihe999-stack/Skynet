// Package response 提供 Skynet 平台的 HTTP JSON 响应构建工具。
//
// 该包基于 Gin 框架封装了统一的 HTTP 响应格式，
// 所有 API 接口的响应都通过此包的函数构建，确保响应格式的一致性。
// 支持成功响应、各类错误响应以及分页数据响应。
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 是 HTTP API 的统一 JSON 响应结构。
//
// Skynet 平台所有 HTTP 接口均使用此结构作为响应体，
// 确保客户端可以用统一的方式解析处理响应数据。
//
// 字段说明：
//   - Code: 业务状态码。0 表示成功，非 0 值表示错误（通常与 HTTP 状态码一致）。
//   - Message: 响应消息描述，如 "ok"、"created" 或具体的错误信息。
//   - Data: 响应数据载荷，仅在成功响应中包含具体数据，错误响应时为空。
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Success 发送 HTTP 200 成功响应。
//
// 构建一个 Code 为 0、Message 为 "ok" 的标准成功响应，
// 并将数据附加在 Data 字段中返回给客户端。
//
// 参数：
//   - c: Gin 上下文对象，用于写入 HTTP 响应。
//   - data: 响应数据，可以是任意类型，将被序列化为 JSON。
func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{Code: 0, Message: "ok", Data: data})
}

// Created 发送 HTTP 201 资源创建成功响应。
//
// 通常在 POST 请求成功创建新资源后使用，
// 构建一个 Code 为 0、Message 为 "created" 的响应。
//
// 参数：
//   - c: Gin 上下文对象，用于写入 HTTP 响应。
//   - data: 新创建的资源数据，将被序列化为 JSON 返回给客户端。
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Response{Code: 0, Message: "created", Data: data})
}

// Error 发送指定 HTTP 状态码的错误响应。
//
// 这是所有错误响应的基础函数，其他具体错误函数（如 BadRequest、NotFound 等）
// 都是对此函数的便捷封装。Code 字段设置为与 HTTP 状态码相同的值。
//
// 参数：
//   - c: Gin 上下文对象，用于写入 HTTP 响应。
//   - httpStatus: HTTP 状态码（如 400、401、404、500 等）。
//   - message: 错误描述信息，帮助客户端理解错误原因。
func Error(c *gin.Context, httpStatus int, message string) {
	c.JSON(httpStatus, Response{Code: httpStatus, Message: message})
}

// BadRequest 发送 HTTP 400 错误请求响应。
//
// 当客户端请求参数不合法、缺少必要参数或格式错误时使用。
//
// 参数：
//   - c: Gin 上下文对象，用于写入 HTTP 响应。
//   - message: 错误描述信息，说明请求参数的具体问题。
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, message)
}

// Unauthorized 发送 HTTP 401 未授权响应。
//
// 当请求未携带有效的认证信息（如 API Key、Token 缺失或过期）时使用。
//
// 参数：
//   - c: Gin 上下文对象，用于写入 HTTP 响应。
//   - message: 错误描述信息，说明认证失败的原因。
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, message)
}

// Forbidden 发送 HTTP 403 禁止访问响应。
//
// 当请求已通过认证但没有足够权限访问目标资源时使用。
//
// 参数：
//   - c: Gin 上下文对象，用于写入 HTTP 响应。
//   - message: 错误描述信息，说明权限不足的原因。
func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, message)
}

// NotFound 发送 HTTP 404 资源未找到响应。
//
// 当请求的资源不存在时使用。
//
// 参数：
//   - c: Gin 上下文对象，用于写入 HTTP 响应。
//   - message: 错误描述信息，说明未找到的资源信息。
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, message)
}

// InternalServerError 发送 HTTP 500 服务器内部错误响应。
//
// 当服务器处理请求过程中发生未预期的内部错误时使用。
// 在生产环境中应避免将详细的内部错误信息直接暴露给客户端。
//
// 参数：
//   - c: Gin 上下文对象，用于写入 HTTP 响应。
//   - message: 错误描述信息。
func InternalServerError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, message)
}

// PaginatedData 是分页查询结果的数据结构。
//
// 用于封装列表类接口的分页响应数据，
// 作为 Response.Data 的值嵌套在统一响应结构中返回。
//
// 字段说明：
//   - Items: 当前页的数据列表，类型由具体业务决定。
//   - Total: 符合查询条件的数据总条数（不受分页限制）。
//   - Page: 当前页码，从 1 开始。
//   - PageSize: 每页数据条数。
//   - TotalPages: 总页数，根据 Total 和 PageSize 计算得出。
type PaginatedData struct {
	Items      any   `json:"items"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
}

// Paginated 发送 HTTP 200 分页数据响应。
//
// 该函数自动计算总页数，并将分页信息和数据列表封装为 PaginatedData 结构，
// 通过 Success 函数以标准成功响应格式返回给客户端。
//
// 参数：
//   - c: Gin 上下文对象，用于写入 HTTP 响应。
//   - items: 当前页的数据列表。
//   - total: 符合查询条件的数据总条数。
//   - page: 当前页码。
//   - pageSize: 每页数据条数。
func Paginated(c *gin.Context, items any, total int64, page, pageSize int) {
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}
	Success(c, PaginatedData{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}
