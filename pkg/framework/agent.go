// Package framework 提供 Skynet Agent 开发框架的核心 SDK。
//
// 开发者通过该包创建 Agent 实例、注册 Skill（技能）、调用 Run() 启动服务。
// 框架会自动处理配置加载、网络连接、心跳保活、请求路由等底层细节，
// 使开发者只需关注业务逻辑（Skill Handler）的实现。
//
// 典型使用流程：New() → Register(skill) → Run()。
// 框架根据配置自动选择开发模式（本地 HTTP 服务器）或生产模式（WebSocket 反向通道连接 Gateway）。
package framework

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/skynetplatform/skynet/pkg/logger"
)

// Agent 是 Skynet Agent 的核心入口结构体。
//
// Agent 代表一个可注册多个 Skill 的智能体实例。它负责管理 Agent 的完整生命周期：
// 加载配置 → 注册 Skill → 启动运行（开发模式或生产模式）。
//
// 字段说明：
//   - config: Agent 的配置信息，从 skynet.yaml 加载
//   - skills: 已注册的 Skill 集合，以 Skill 名称为键
type Agent struct {
	config *AgentConfig
	skills map[string]Skill
}

// New 创建一个新的 Agent 实例，自动从 skynet.yaml 配置文件加载配置。
//
// 该函数是创建 Agent 的主要入口。它会按照优先级搜索配置文件
// （先查找环境特定的 skynet.{ENV}.yaml，再查找默认的 skynet.yaml），
// 并初始化 Agent 的内部状态。
//
// 如果配置文件加载失败，程序会通过 logger.Fatalf 直接终止。
//
// 返回值：
//   - *Agent: 初始化完成的 Agent 实例，可立即注册 Skill 并运行
func New() *Agent {
	cfg, err := LoadConfig()
	if err != nil {
		logger.Fatalf("Failed to load skynet.yaml: %v", err)
	}
	return &Agent{
		config: cfg,
		skills: make(map[string]Skill),
	}
}

// NewWithConfig 使用给定的配置创建一个新的 Agent 实例。
//
// 与 New() 不同，该函数不会自动加载 skynet.yaml，而是直接使用传入的配置。
// 适用于测试场景或需要以编程方式构建配置的场合。
//
// 参数：
//   - cfg: Agent 配置，包含 Agent 信息、网络配置、服务器配置等
//
// 返回值：
//   - *Agent: 初始化完成的 Agent 实例
func NewWithConfig(cfg *AgentConfig) *Agent {
	return &Agent{
		config: cfg,
		skills: make(map[string]Skill),
	}
}

// Register 向当前 Agent 注册一个 Skill（技能）。
//
// Skill 是 Agent 对外暴露的能力单元，每个 Skill 必须有唯一的名称和对应的处理函数。
// 注册后的 Skill 会在 Agent 运行时自动对外提供服务（通过本地 HTTP 或 WebSocket 通道）。
//
// 参数：
//   - skill: 要注册的 Skill 定义，必须包含非空的 Name 和非 nil 的 Handler
//
// 如果 Skill 名称为空或 Handler 为 nil，程序会通过 logger.Fatal/Fatalf 直接终止。
func (a *Agent) Register(skill Skill) {
	if skill.Name == "" {
		logger.Fatal("Skill name cannot be empty")
	}
	if skill.Handler == nil {
		logger.Fatalf("Skill '%s' has no handler", skill.Name)
	}
	a.skills[skill.Name] = skill
	logger.Debugf("Registered skill: %s", skill.Name)
}

// Run 启动 Agent 运行。
//
// 该方法是 Agent 生命周期的最后一步。它根据配置自动选择运行模式：
//   - 开发模式（dev）：未配置 registry 或 api_key 时，启动本地 HTTP 服务器用于测试
//   - 生产模式（prod）：配置完整时，通过 WebSocket 反向通道连接 Skynet Gateway
//
// 该方法为阻塞调用，在 Agent 停止前不会返回。
// 如果没有注册任何 Skill，程序会通过 logger.Fatal 直接终止。
func (a *Agent) Run() {
	if len(a.skills) == 0 {
		logger.Fatal("No skills registered")
	}

	if a.config.IsDevMode() {
		logger.Info("Dev mode: no registry or API key configured, starting local server")
		a.runDev()
	} else {
		a.runProd()
	}
}

// runDev 以开发模式启动 Agent。
//
// 生成 Agent Card（能力名片），然后启动本地 HTTP 服务器。
// 开发模式不连接 Skynet 网络，仅提供本地 REST API 用于调试和测试 Skill。
// 同时注册系统信号（SIGINT、SIGTERM）监听器，实现优雅关闭。
func (a *Agent) runDev() {
	card := GenerateCard(a.config, a.skillList())
	server := NewLocalServer(a.config.Server.Port, card, a.skills)
	if err := server.Start(); err != nil {
		logger.Fatalf("Local server error: %v", err)
	}

	// 优雅关闭：监听系统终止信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Println()
	logger.Info("Shutting down dev server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Errorf("Dev server shutdown error: %v", err)
	}
}

// runProd 以生产模式启动 Agent。
//
// 生成 Agent Card，将 Registry URL 转换为 WebSocket 协议地址，
// 创建 TunnelClient 并通过 WebSocket 反向通道连接 Gateway。
// 同时注册系统信号（SIGINT、SIGTERM）监听器，实现优雅关闭。
// 连接断开后会自动重连（指数退避策略）。
func (a *Agent) runProd() {
	agentCard := GenerateCard(a.config, a.skillList())

	gatewayURL := strings.TrimSuffix(a.config.Network.Registry, "/")
	gatewayURL = strings.Replace(gatewayURL, "https://", "wss://", 1)
	gatewayURL = strings.Replace(gatewayURL, "http://", "ws://", 1)

	tunnel := NewTunnelClient(gatewayURL, agentCard, a.skills)

	// 优雅关闭：监听系统终止信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println()
		logger.Info("Shutting down...")
		tunnel.Close()
	}()

	tunnel.ConnectWithRetry()
}

// skillList 将已注册的 Skill map 转换为 Skill 切片。
//
// 该方法在生成 Agent Card 时使用，将 map 结构的 skills 转换为有序的切片形式，
// 便于遍历和序列化。
//
// 返回值：
//   - []Skill: 包含所有已注册 Skill 的切片
func (a *Agent) skillList() []Skill {
	skills := make([]Skill, 0, len(a.skills))
	for _, s := range a.skills {
		skills = append(skills, s)
	}
	return skills
}
