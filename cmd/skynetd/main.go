// Package main 实现 skynetd 平台服务端守护进程的入口。
// skynetd 是 Skynet 平台的核心服务端程序，负责运行以下关键模块：
//   - Registry（注册中心）：管理 Agent 的注册、发现和心跳监测
//   - Gateway（网关）：处理 Skill 调用请求的路由转发，管理 Agent 的 WebSocket 反向通道
//   - AuthZ（认证授权）：处理用户和 Agent 的身份验证与权限控制
//   - API 服务：提供 RESTful API 和 WebSocket 端点供 Agent 和客户端交互
//
// skynetd 启动流程：加载配置 -> 连接数据库 -> 初始化各模块 -> 启动 HTTP 服务 -> 优雅关闭
package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/eyrihe999-stack/Skynet/internal/api"
	"github.com/eyrihe999-stack/Skynet/internal/api/handler"
	"github.com/eyrihe999-stack/Skynet/internal/authz"
	"github.com/eyrihe999-stack/Skynet/internal/config"
	"github.com/eyrihe999-stack/Skynet/internal/gateway"
	"github.com/eyrihe999-stack/Skynet/internal/model"
	"github.com/eyrihe999-stack/Skynet/internal/registry"
	"github.com/eyrihe999-stack/Skynet/internal/store"
	"github.com/eyrihe999-stack/Skynet/pkg/database"
	"github.com/eyrihe999-stack/Skynet-sdk/logger"
)

// main 是 skynetd 平台守护进程的入口函数。
// 该函数按顺序完成以下 10 个步骤来启动整个 Skynet 平台服务：
//  1. 加载配置：解析命令行参数 --config 指定的配置文件路径，加载服务端配置
//  2. 连接数据库：使用配置中的数据库参数建立 MySQL 连接
//  3. 数据库迁移：自动迁移 User、Agent、Capability、Invocation 等数据模型（开发便利性）
//  4. 创建仓储层：初始化 Agent、Capability、Invocation 的数据库访问层（Repository）
//  5. 创建服务层：初始化认证服务（AuthZ）、注册中心服务（Registry）、隧道管理器（TunnelManager）和网关服务（Gateway）
//  6. 创建处理器：初始化各 HTTP/WebSocket 请求处理器（Handler），连接服务层和 API 层
//  7. 配置路由：使用 Gin 框架设置 HTTP 路由，将各端点映射到对应的处理器
//  8. 启动心跳监控：在后台 goroutine 中运行 Registry 的心跳监控，定期检查 Agent 在线状态
//  9. 启动 HTTP 服务：在配置的监听地址上启动 HTTP 服务器
//  10. 优雅关闭：监听系统信号（SIGINT/SIGTERM），收到后停止心跳监控，
//     在 10 秒超时内优雅关闭 HTTP 服务器，确保正在处理的请求能够完成
func main() {
	configPath := flag.String("config", "", "Path to config file (default: auto-resolve)")
	flag.Parse()

	// 1. Load config
	cfg := config.LoadFrom(*configPath)
	logger.SetLevel(cfg.LogLevel)
	logger.Info("Starting Skynet Platform...")

	// 2. Connect to MySQL
	db, err := database.NewMySQL(cfg.Database)
	if err != nil {
		logger.Fatalf("Failed to connect to MySQL: %v", err)
	}

	// 3. Auto-migrate (dev convenience)
	if err := db.AutoMigrate(
		&model.User{},
		&model.Agent{},
		&model.Capability{},
		&model.Invocation{},
		&model.Approval{},
		&model.PermissionRule{},
		&model.TaskMessage{},
		&model.CapabilityEmbedding{},
	); err != nil {
		logger.Fatalf("Auto-migrate failed: %v", err)
	}

	// 4. Create repositories
	agentRepo := store.NewAgentRepo(db)
	capRepo := store.NewCapabilityRepo(db)
	invRepo := store.NewInvocationRepo(db)
	approvalRepo := store.NewApprovalRepo(db)
	permRepo := store.NewPermissionRepo(db)
	taskMsgRepo := store.NewTaskMessageRepo(db)
	embeddingRepo := store.NewEmbeddingRepo(db)

	// 5. Create services
	embeddingClient := registry.NewEmbeddingClient(cfg.Embedding)
	authSvc := authz.NewService(db, cfg.JWT.Secret)
	registrySvc := registry.NewService(agentRepo, capRepo, embeddingRepo, embeddingClient)
	connMgr := gateway.NewConnectionManager()
	callbackMgr := gateway.NewCallbackManager(cfg.Server.ExternalURL)
	eventBus := gateway.NewEventBus()
	rateLimiter := gateway.NewRateLimiter()
	taskSessions := gateway.NewTaskSessionManager()
	gatewaySvc := gateway.NewService(connMgr, invRepo, capRepo, agentRepo, permRepo, rateLimiter, approvalRepo, taskSessions, taskMsgRepo, eventBus)

	// 6. Create handlers
	authHandler := authz.NewHandler(authSvc)
	registryHandler := handler.NewRegistryHandler(registrySvc, connMgr, callbackMgr, eventBus)
	invokeHandler := handler.NewInvokeHandler(gatewaySvc)
	tunnelHandler := handler.NewTunnelHandler(connMgr, registrySvc, authSvc, eventBus)
	invocationHandler := handler.NewInvocationHandler(invRepo)
	approvalHandler := handler.NewApprovalHandler(approvalRepo)
	permissionHandler := handler.NewPermissionHandler(permRepo, agentRepo)
	taskHandler := handler.NewTaskHandler(gatewaySvc)
	eventsHandler := handler.NewEventsHandler(eventBus, authSvc)

	// 7. Setup router
	gin.SetMode(gin.ReleaseMode)
	if cfg.LogLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	}

	router := api.SetupRouter(api.Deps{
		AuthSvc:           authSvc,
		AuthHandler:       authHandler,
		RegistryHandler:   registryHandler,
		InvokeHandler:     invokeHandler,
		TunnelHandler:     tunnelHandler,
		InvocationHandler: invocationHandler,
		ApprovalHandler:   approvalHandler,
		PermissionHandler: permissionHandler,
		TaskHandler:       taskHandler,
		EventsHandler:     eventsHandler,
		CallbackHandler:   handler.NewCallbackHandler(callbackMgr),
	})

	// 8. Cleanup stale tasks from previous run
	if count, err := invRepo.CleanupStale(); err != nil {
		logger.Errorf("Failed to cleanup stale invocations: %v", err)
	} else if count > 0 {
		logger.Infof("Cleaned up %d stale invocations from previous run", count)
	}

	// 9. Start heartbeat monitor
	stopHeartbeat := make(chan struct{})
	go registrySvc.StartHeartbeatMonitor(stopHeartbeat)
	rateLimiter.StartCleanup(5*time.Minute, stopHeartbeat)

	// 9. Start HTTP server
	srv := &http.Server{
		Addr:    cfg.Server.ListenAddr,
		Handler: router,
	}

	go func() {
		logger.Infof("Skynet Platform listening on %s", cfg.Server.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server error: %v", err)
		}
	}()

	// 10. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down...")
	close(stopHeartbeat)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Errorf("Server shutdown error: %v", err)
	}

	logger.Info("Skynet Platform stopped")
}
