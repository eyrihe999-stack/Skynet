package framework

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/skynetplatform/skynet/pkg/logger"
	"github.com/skynetplatform/skynet/pkg/protocol"
)

// LocalServer 是本地开发 HTTP 服务器，用于在不连接 Skynet 网络的情况下测试 Agent 的 Skill。
//
// LocalServer 是开发模式下的核心组件，基于 Gin 框架提供 REST API，
// 使开发者可以通过 HTTP 请求直接调用和调试 Skill，无需部署到 Skynet 网络。
//
// 提供以下 API 端点：
//   - GET  /agent-card      — 查看 Agent Card（能力名片）
//   - GET  /skills          — 列出所有已注册的 Skill 名称
//   - POST /skills/:name    — 调用指定的 Skill
//
// 字段说明：
//   - port: HTTP 服务器监听端口
//   - card: Agent Card，通过 /agent-card 端点返回
//   - skills: 已注册的 Skill 集合，通过 /skills/:name 端点调用
type LocalServer struct {
	port   int
	card   protocol.AgentCard
	skills map[string]Skill
	server *http.Server
}

// NewLocalServer 创建一个新的本地开发服务器实例。
//
// 该函数仅初始化 LocalServer 的内部状态，不会立即启动 HTTP 服务器。
// 需要调用 Start() 来实际启动监听和服务。
//
// 参数：
//   - port: HTTP 服务器监听端口（通常从配置文件的 server.port 获取，默认 9100）
//   - card: Agent Card，包含 Agent 的身份和能力信息
//   - skills: 已注册的 Skill 集合，以 Skill 名称为键
//
// 返回值：
//   - *LocalServer: 初始化完成的本地服务器实例
func NewLocalServer(port int, card protocol.AgentCard, skills map[string]Skill) *LocalServer {
	return &LocalServer{port: port, card: card, skills: skills}
}

// Start 启动本地 HTTP 开发服务器。
//
// 该方法为非阻塞调用，在后台 goroutine 中启动 HTTP 服务器。
// 服务器以 Gin 的 DebugMode 运行，提供详细的请求日志。
// 调用 Shutdown() 可优雅关闭服务器。
//
// 注册的 API 端点：
//   - GET  /agent-card: 返回 Agent Card 的 JSON 表示，包含 Agent 的完整能力描述
//   - GET  /skills: 返回所有已注册 Skill 名称的 JSON 数组
//   - POST /skills/:name: 调用指定名称的 Skill，请求体为 JSON 格式的输入参数，
//     支持空请求体（默认传入空 JSON 对象）。成功时返回 {"output": ...}，
//     Skill 未找到返回 404，执行失败返回 500
//
// 返回值：
//   - error: 服务器启动过程中发生的错误
func (s *LocalServer) Start() error {
	gin.SetMode(gin.DebugMode)
	r := gin.Default()

	r.GET("/agent-card", func(c *gin.Context) {
		c.JSON(http.StatusOK, s.card)
	})

	r.GET("/skills", func(c *gin.Context) {
		names := make([]string, 0, len(s.skills))
		for name := range s.skills {
			names = append(names, name)
		}
		c.JSON(http.StatusOK, gin.H{"skills": names})
	})

	r.POST("/skills/:name", func(c *gin.Context) {
		// panic recovery：捕获 Handler 内部的 panic，返回 500 而非崩溃进程
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("Skill handler panic (skill=%s): %v", c.Param("name"), r)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("internal error: handler panic: %v", r),
				})
			}
		}()

		name := c.Param("name")
		skill, ok := s.skills[name]
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("skill '%s' not found", name)})
			return
		}

		var rawInput json.RawMessage
		if err := c.ShouldBindJSON(&rawInput); err != nil {
			// 允许空请求体，默认使用空 JSON 对象
			rawInput = json.RawMessage(`{}`)
		}

		// 输入校验：按 Skill 定义的 Input Schema 校验必填字段和类型
		input := NewInput(rawInput)
		if skill.Input != nil {
			if err := skill.Input.Validate(input.Raw()); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("input validation failed: %s", err.Error())})
				return
			}
		}

		ctx := Context{RequestID: "local"}

		output, err := skill.Handler(ctx, input)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 检查是否为多轮对话追问
		if needInput, ok := output.(*NeedInputResult); ok {
			c.JSON(http.StatusOK, gin.H{"status": "input_required", "question": needInput.Question})
			return
		}

		c.JSON(http.StatusOK, gin.H{"output": output})
	})

	addr := fmt.Sprintf(":%d", s.port)
	s.server = &http.Server{
		Addr:    addr,
		Handler: r,
	}

	logger.Infof("Local dev server starting on %s", addr)
	logger.Infof("  GET  /agent-card       — view agent card")
	logger.Infof("  GET  /skills           — list skills")
	logger.Infof("  POST /skills/:name     — invoke a skill")

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("Local server error: %v", err)
		}
	}()

	return nil
}

// Shutdown 优雅关闭本地 HTTP 开发服务器。
//
// 该方法会停止接受新的连接，等待已有请求处理完毕后关闭服务器。
// 超时时间由传入的 context 控制。
//
// 参数：
//   - ctx: 用于控制关闭超时的上下文
//
// 返回值：
//   - error: 关闭过程中发生的错误
func (s *LocalServer) Shutdown(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}
