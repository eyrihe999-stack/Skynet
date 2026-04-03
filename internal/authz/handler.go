package authz

import (
	"github.com/gin-gonic/gin"
	"github.com/eyrihe999-stack/Skynet/pkg/response"
)

// Handler 是认证模块的 HTTP 处理器，负责处理注册、登录和用户信息获取等 HTTP 请求。
//
// 它依赖 Service 完成实际的业务逻辑，自身仅负责请求参数解析、
// 调用服务层方法以及将结果格式化为统一的 HTTP 响应。
// 在 Skynet 架构中，Handler 的路由通常注册在公开路由组中（注册、登录），
// 或注册在需要认证的路由组中（获取用户信息）。
type Handler struct {
	// svc 是认证服务实例，Handler 将所有业务逻辑委托给该服务处理。
	svc *Service
}

// NewHandler 创建并返回一个新的认证 HTTP 处理器实例。
//
// 参数：
//   - svc: 认证服务实例，提供注册、登录等核心业务能力。
//
// 返回值：
//   - *Handler: 初始化完成的 HTTP 处理器实例指针。
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// registerRequest 是用户注册接口的请求体结构，用于绑定和校验 JSON 请求参数。
//
// 字段校验规则：
//   - Email: 必填，需为合法邮箱格式。
//   - Password: 必填，最少 6 个字符。
//   - DisplayName: 必填，用户显示名称。
type registerRequest struct {
	// Email 是用户邮箱地址，作为登录凭据使用，需全局唯一。
	Email string `json:"email" binding:"required,email"`
	// Password 是用户密码，最少 6 个字符，将以 bcrypt 哈希存储。
	Password string `json:"password" binding:"required,min=6"`
	// DisplayName 是用户的显示名称，用于界面展示。
	DisplayName string `json:"display_name" binding:"required"`
}

// Register 处理用户注册的 HTTP 请求。
//
// 该方法从请求体解析注册参数，调用 Service.Register 完成用户创建，
// 并返回包含用户信息和 API Key 的响应。
//
// HTTP 方法: POST
// 请求体: registerRequest（JSON 格式）
//
// 响应：
//   - 201 Created: 注册成功，返回 RegisterResult（包含用户信息和明文 API Key）。
//   - 400 Bad Request: 参数校验失败或邮箱已被注册。
//   - 500 Internal Server Error: 服务端内部错误。
//
// 参数：
//   - c: Gin 上下文，用于读取请求和写入响应。
func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.svc.Register(req.Email, req.Password, req.DisplayName)
	if err != nil {
		if err == ErrEmailExists {
			response.BadRequest(c, "email already registered")
			return
		}
		response.InternalServerError(c, "registration failed")
		return
	}

	response.Created(c, result)
}

// loginRequest 是用户登录接口的请求体结构，用于绑定和校验 JSON 请求参数。
//
// 字段校验规则：
//   - Email: 必填，需为合法邮箱格式。
//   - Password: 必填。
type loginRequest struct {
	// Email 是用户邮箱地址。
	Email string `json:"email" binding:"required,email"`
	// Password 是用户密码。
	Password string `json:"password" binding:"required"`
}

// Login 处理用户登录的 HTTP 请求。
//
// 该方法从请求体解析登录参数，调用 Service.Login 验证身份，
// 成功后返回签发的 JWT 令牌和用户信息。
// 出于安全考虑，无论是用户不存在还是密码错误，统一返回 "invalid email or password"，
// 避免泄露用户是否已注册的信息。
//
// HTTP 方法: POST
// 请求体: loginRequest（JSON 格式）
//
// 响应：
//   - 200 OK: 登录成功，返回 LoginResult（包含 JWT 令牌和用户信息）。
//   - 400 Bad Request: 参数校验失败。
//   - 401 Unauthorized: 邮箱或密码错误。
//   - 500 Internal Server Error: 服务端内部错误。
//
// 参数：
//   - c: Gin 上下文，用于读取请求和写入响应。
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.svc.Login(req.Email, req.Password)
	if err != nil {
		if err == ErrUserNotFound || err == ErrInvalidPassword {
			response.Unauthorized(c, "invalid email or password")
			return
		}
		response.InternalServerError(c, "login failed")
		return
	}

	response.Success(c, result)
}

// RegenerateKey 重新生成当前用户的 API Key。
// POST /api/v1/auth/regenerate-key
func (h *Handler) RegenerateKey(c *gin.Context) {
	user := GetCurrentUser(c)
	if user == nil {
		response.Unauthorized(c, "not authenticated")
		return
	}
	apiKey, err := h.svc.RegenerateAPIKey(user.ID)
	if err != nil {
		response.InternalServerError(c, "failed to regenerate API key")
		return
	}
	response.Success(c, gin.H{"api_key": apiKey})
}

// Profile 处理获取当前登录用户信息的 HTTP 请求。
//
// 该方法从 Gin 上下文中获取经中间件认证后注入的用户信息，
// 并将其作为响应返回。此接口需要配合 AuthRequired 中间件使用，
// 以确保请求已通过身份验证。
//
// HTTP 方法: GET
//
// 响应：
//   - 200 OK: 返回当前登录用户的完整信息（model.User）。
//   - 401 Unauthorized: 用户未通过身份验证。
//
// 参数：
//   - c: Gin 上下文，用于读取请求和写入响应。
func (h *Handler) Profile(c *gin.Context) {
	user := GetCurrentUser(c)
	if user == nil {
		response.Unauthorized(c, "not authenticated")
		return
	}
	response.Success(c, user)
}
