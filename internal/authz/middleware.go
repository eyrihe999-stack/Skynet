package authz

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/eyrihe999-stack/Skynet/internal/model"
	"github.com/eyrihe999-stack/Skynet/pkg/response"
)

// UserContextKey 是在 Gin 上下文中存储已认证用户信息的键名。
//
// 中间件验证通过后，会使用此键将 *model.User 存入 Gin 上下文，
// 后续的 Handler 可通过 GetCurrentUser 函数取出。
const UserContextKey = "current_user"

// AuthRequired 是 Gin 认证中间件，用于拦截请求并验证客户端身份。
//
// 该中间件支持两种认证方式，按以下优先级依次尝试：
//  1. API Key 认证：检查请求头 "X-API-Key"，适用于 Agent 建立 WebSocket 隧道
//     以及程序化 API 调用场景。
//  2. JWT Bearer Token 认证：检查请求头 "Authorization: Bearer <token>"，
//     适用于用户通过浏览器或客户端登录后的会话访问。
//
// 验证通过后，将用户信息存入 Gin 上下文（键为 UserContextKey），供后续 Handler 使用。
// 验证失败或未提供任何凭据时，返回 401 Unauthorized 并中止请求链。
//
// 参数：
//   - svc: 认证服务实例，用于执行 API Key 和 JWT 的实际验证逻辑。
//
// 返回值：
//   - gin.HandlerFunc: 可注册到 Gin 路由组的中间件函数。
func AuthRequired(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 优先尝试 API Key 认证，检查 X-API-Key 请求头
		apiKey := c.GetHeader("X-API-Key")
		if apiKey != "" {
			user, err := svc.ValidateAPIKey(apiKey)
			if err != nil {
				response.Unauthorized(c, "invalid API key")
				c.Abort()
				return
			}
			c.Set(UserContextKey, user)
			c.Next()
			return
		}

		// 其次尝试 JWT Bearer Token 认证，检查 Authorization 请求头
		auth := c.GetHeader("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			tokenStr := strings.TrimPrefix(auth, "Bearer ")
			user, err := svc.ValidateJWT(tokenStr)
			if err != nil {
				response.Unauthorized(c, "invalid token")
				c.Abort()
				return
			}
			c.Set(UserContextKey, user)
			c.Next()
			return
		}

		// 未提供任何认证凭据，返回 401 并中止请求
		response.Unauthorized(c, "authentication required")
		c.Abort()
	}
}

// GetCurrentUser 从 Gin 上下文中获取当前已认证的用户信息。
//
// 该函数通常在经过 AuthRequired 中间件保护的 Handler 中调用，
// 用于获取中间件注入的用户模型。如果上下文中不存在用户信息
// （例如该路由未使用认证中间件），则返回 nil。
//
// 参数：
//   - c: Gin 上下文，中间件验证通过后会在其中存储用户信息。
//
// 返回值：
//   - *model.User: 已认证的用户模型指针；若未找到或类型断言失败则返回 nil。
func GetCurrentUser(c *gin.Context) *model.User {
	v, exists := c.Get(UserContextKey)
	if !exists {
		return nil
	}
	user, ok := v.(*model.User)
	if !ok {
		return nil
	}
	return user
}
