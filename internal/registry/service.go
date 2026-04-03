// Package registry 提供 Agent 注册中心的核心服务。
// 该包是 Skynet 平台中 Agent 注册、发现和生命周期管理的核心业务层，
// 负责处理 Agent 的注册/注销、心跳检测、信息查询和能力搜索等功能。
// Registry 层依赖 Store 层（AgentRepo、CapabilityRepo）完成数据持久化操作。
package registry

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/eyrihe999-stack/Skynet/internal/model"
	"github.com/eyrihe999-stack/Skynet/internal/store"
	"github.com/eyrihe999-stack/Skynet-sdk/logger"
	"github.com/eyrihe999-stack/Skynet-sdk/protocol"
)

// Service 是注册中心的核心服务结构体，提供 Agent 注册、注销、心跳、发现和能力搜索等功能。
// 它是 Registry 层的唯一入口，协调 AgentRepo 和 CapabilityRepo 两个数据仓库完成业务逻辑。
//
// 字段说明：
//   - agentRepo: Agent 数据仓库，负责 agents 表的 CRUD 操作
//   - capRepo: 能力数据仓库，负责 capabilities 表的 CRUD 和全文搜索操作
//   - embeddingRepo: Embedding 数据仓库，负责 capability_embeddings 表的读写操作
//   - embeddingClient: Embedding API 客户端，用于生成文本向量（可为 nil 表示未启用）
type Service struct {
	agentRepo       *store.AgentRepo
	capRepo         *store.CapabilityRepo
	embeddingRepo   *store.EmbeddingRepo
	embeddingClient *EmbeddingClient
}

// NewService 创建并返回一个新的注册中心服务实例。
// 这是 Service 的构造函数，通过依赖注入的方式接收所需的数据仓库。
//
// 参数：
//   - agentRepo: Agent 数据仓库实例，用于 Agent 相关的数据库操作
//   - capRepo: 能力数据仓库实例，用于 Capability 相关的数据库操作
//   - embeddingRepo: Embedding 数据仓库实例，用于 capability embedding 的读写操作
//   - embeddingClient: Embedding API 客户端实例，用于生成文本向量（可为 nil）
//
// 返回值：
//   - *Service: 初始化完成的注册中心服务实例
func NewService(agentRepo *store.AgentRepo, capRepo *store.CapabilityRepo, embeddingRepo *store.EmbeddingRepo, embeddingClient *EmbeddingClient) *Service {
	return &Service{
		agentRepo:       agentRepo,
		capRepo:         capRepo,
		embeddingRepo:   embeddingRepo,
		embeddingClient: embeddingClient,
	}
}

// RegisterAgent 根据 AgentCard 协议数据创建或更新一个 Agent 及其能力列表。
// 该方法是 Agent 注册流程的核心入口，支持首次注册和重复注册两种场景：
//   - 首次注册：生成 agent_secret 并返回给调用方，用于后续身份验证
//   - 重复注册：复用已有的 secret hash，更新 Agent 信息和能力列表
//
// 参数：
//   - card: 符合 A2A 协议的 AgentCard，包含 Agent 的基本信息和能力声明
//   - ownerID: Agent 所有者的用户 ID，用于标识 Agent 的归属
//
// 返回值：
//   - agentSecret: 首次注册时返回生成的 agent secret，重复注册时返回空字符串
//   - err: 注册过程中的错误，包括数据库操作失败、密钥生成失败等
func (s *Service) RegisterAgent(card protocol.AgentCard, ownerID uint64, endpointURL ...string) (agentSecret string, err error) {
	existing, _ := s.agentRepo.FindByAgentID(card.AgentID)

	now := time.Now()
	var secretHash string

	if existing == nil {
		// 首次注册 — 生成 agent secret，明文存储
		agentSecret = generateAgentSecret()
		secretHash = agentSecret
	} else {
		// 重复注册 — 复用已有的 secret
		secretHash = existing.AgentSecretHash
	}

	// 将协议层的 DataPolicy 转换为数据模型层的 JSONMap 格式
	var dataPolicy model.JSONMap
	if card.DataPolicy != nil {
		b, _ := json.Marshal(card.DataPolicy)
		json.Unmarshal(b, &dataPolicy)
	}

	// 解析可选的 endpointURL 参数（direct/webhook 模式使用）
	var epURL string
	if len(endpointURL) > 0 {
		epURL = endpointURL[0]
	}

	// 构建 Agent 数据模型并执行 Upsert 操作
	agent := &model.Agent{
		AgentID:          card.AgentID,
		OwnerID:          ownerID,
		DisplayName:      card.DisplayName,
		Description:      card.Description,
		ConnectionMode:   card.ConnectionMode,
		EndpointURL:      epURL,
		AgentSecretHash:  secretHash,
		DataPolicy:       dataPolicy,
		Status:           "online",
		LastHeartbeatAt:  &now,
		FrameworkVersion: card.FrameworkVersion,
		Version:          card.Version,
	}

	if err := s.agentRepo.Upsert(agent); err != nil {
		return "", err
	}

	// 将协议层的能力声明转换为数据模型，并批量 Upsert 到数据库
	caps := make([]model.Capability, len(card.Capabilities))
	for i, c := range card.Capabilities {
		caps[i] = model.Capability{
			AgentID:            card.AgentID,
			Name:               c.Name,
			DisplayName:        c.DisplayName,
			Description:        c.Description,
			Category:           c.Category,
			Tags:               model.JSONArray(c.Tags),
			InputSchema:        model.JSONRaw(c.InputSchema),
			OutputSchema:       model.JSONRaw(c.OutputSchema),
			Visibility:         c.Visibility,
			ApprovalMode:       c.ApprovalMode,
			MultiTurn:          c.MultiTurn,
			EstimatedLatencyMs: &c.EstimatedLatencyMs,
		}
	}

	if err := s.capRepo.BulkUpsert(card.AgentID, caps); err != nil {
		return "", err
	}

	// 异步生成 capability embedding（不阻塞注册流程）
	if s.embeddingClient != nil && s.embeddingClient.Enabled() {
		go s.generateEmbeddings(card.AgentID)
	}

	logger.Infof("Agent registered: %s (%d capabilities)", card.AgentID, len(caps))
	return agentSecret, nil
}

// GetAgentSecret 返回指定 Agent 的 secret（用于 webhook 签名等场景）。
func (s *Service) GetAgentSecret(agentID string) (string, error) {
	agent, err := s.agentRepo.FindByAgentID(agentID)
	if err != nil {
		return "", err
	}
	return agent.AgentSecretHash, nil
}

// UnregisterAgent 将指定 Agent 的状态设置为离线，实现逻辑注销。
// 注销后 Agent 不会从数据库中物理删除，而是通过状态标记为 "offline"，
// 使其不再出现在能力搜索结果中，但仍可通过 ID 查询到历史信息。
//
// 参数：
//   - agentID: 要注销的 Agent 唯一标识符
//
// 返回值：
//   - error: 数据库更新操作的错误，成功时为 nil
func (s *Service) UnregisterAgent(agentID string) error {
	return s.agentRepo.UpdateStatus(agentID, "offline")
}

// Heartbeat 处理 Agent 的心跳请求，更新心跳时间戳并确保 Agent 状态为在线。
// 心跳机制是 Agent 存活检测的基础，Agent 需要定期调用此方法以维持在线状态。
//
// 参数：
//   - agentID: 发送心跳的 Agent 唯一标识符
//
// 返回值：
//   - error: 数据库更新操作的错误，成功时为 nil
func (s *Service) Heartbeat(agentID string) error {
	return s.agentRepo.UpdateHeartbeat(agentID)
}

// GetAgent 根据 Agent ID 查询单个 Agent 的详细信息，包括其能力列表。
// 用于 Agent 详情页展示或跨 Agent 调用时获取目标 Agent 的连接信息。
//
// 参数：
//   - agentID: 要查询的 Agent 唯一标识符
//
// 返回值：
//   - *model.Agent: 查询到的 Agent 实体（含预加载的 Capabilities），未找到时为 nil
//   - error: 查询过程中的错误，未找到记录时返回 gorm.ErrRecordNotFound
func (s *Service) GetAgent(agentID string) (*model.Agent, error) {
	return s.agentRepo.FindByAgentID(agentID)
}

// ListAgents 根据过滤条件分页查询 Agent 列表。
// 支持按状态和所有者 ID 过滤，排除已删除的 Agent，结果按创建时间倒序排列。
//
// 参数：
//   - filter: Agent 查询过滤条件，包含状态、所有者 ID 和分页参数
//
// 返回值：
//   - []model.Agent: 满足条件的 Agent 列表
//   - int64: 满足条件的 Agent 总数（用于分页）
//   - error: 查询过程中的错误
func (s *Service) ListAgents(filter store.AgentFilter) ([]model.Agent, int64, error) {
	return s.agentRepo.List(filter)
}

// SearchCapabilities 根据过滤条件搜索可用的 Agent 能力。
// 仅搜索公开（public）且所属 Agent 在线的能力，支持全文搜索和按分类过滤。
// 当 Embedding 客户端已启用且搜索包含关键词时，会追加语义搜索结果与关键词搜索结果合并。
// 该方法是 Agent 发现机制的核心，允许 Agent 或用户查找平台上可用的服务能力。
//
// 参数：
//   - filter: 能力搜索过滤条件，包含搜索关键词、分类和分页参数
//
// 返回值：
//   - []model.Capability: 满足条件的能力列表
//   - int64: 满足条件的能力总数（用于分页）
//   - error: 搜索过程中的错误
func (s *Service) SearchCapabilities(filter store.CapabilityFilter) ([]model.Capability, int64, error) {
	// 原有关键词搜索
	caps, total, err := s.capRepo.Search(filter)
	if err != nil {
		return nil, 0, err
	}

	// 如果有搜索关键词且 embedding 已启用，追加语义搜索结果
	if filter.Query != "" && s.embeddingClient != nil && s.embeddingClient.Enabled() {
		semanticResults := s.semanticSearch(filter.Query, caps, filter.PageSize)
		if len(semanticResults) > 0 {
			// 合并：关键词结果优先，语义结果补充（去重）
			existing := make(map[uint64]bool)
			for _, c := range caps {
				existing[c.ID] = true
			}
			for _, c := range semanticResults {
				if !existing[c.ID] {
					caps = append(caps, c)
					total++
				}
			}
		}
	}

	return caps, total, nil
}

// generateEmbeddings 为指定 Agent 的所有 capability 生成 embedding 向量。
// 该方法通常在 RegisterAgent 中被异步调用，避免阻塞注册流程。
func (s *Service) generateEmbeddings(agentID string) {
	caps, err := s.capRepo.FindByAgentID(agentID)
	if err != nil {
		logger.Errorf("Failed to load capabilities for embedding: %v", err)
		return
	}
	for _, cap := range caps {
		text := cap.DisplayName + ": " + cap.Description
		embedding, err := s.embeddingClient.Embed(text)
		if err != nil {
			logger.Warnf("Failed to generate embedding for %s/%s: %v", agentID, cap.Name, err)
			continue
		}
		if err := s.embeddingRepo.Upsert(cap.ID, embedding, s.embeddingClient.Model()); err != nil {
			logger.Warnf("Failed to save embedding for %s/%s: %v", agentID, cap.Name, err)
		}
	}
	logger.Debugf("Generated embeddings for %s (%d capabilities)", agentID, len(caps))
}

// semanticSearch 使用向量相似度搜索语义相关的 capability。
// 将查询文本向量化后，与数据库中所有 capability embedding 计算余弦相似度，
// 返回相似度超过阈值（0.5）的结果，按相似度降序排列。
func (s *Service) semanticSearch(query string, exclude []model.Capability, limit int) []model.Capability {
	queryVec, err := s.embeddingClient.Embed(query)
	if err != nil {
		logger.Warnf("Semantic search embedding failed: %v", err)
		return nil
	}

	allEmbeddings, err := s.embeddingRepo.GetAll()
	if err != nil {
		logger.Warnf("Semantic search load embeddings failed: %v", err)
		return nil
	}

	// 排除已有结果的 capability
	excludeIDs := make(map[uint64]bool)
	for _, c := range exclude {
		excludeIDs[c.ID] = true
	}

	// 计算相似度并排序
	type scored struct {
		capID uint64
		score float64
	}
	var results []scored
	for _, emb := range allEmbeddings {
		if excludeIDs[emb.CapabilityID] {
			continue
		}
		vec := store.BytesToFloat32(emb.Embedding)
		sim := CosineSimilarity(queryVec, vec)
		if sim > 0.5 { // 相似度阈值
			results = append(results, scored{emb.CapabilityID, sim})
		}
	}

	// 按相似度降序排序
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// 取前 limit 个
	if len(results) > limit {
		results = results[:limit]
	}

	// 批量查询 capability 详情
	var caps []model.Capability
	for _, r := range results {
		cap, err := s.capRepo.FindByID(r.capID)
		if err == nil {
			caps = append(caps, *cap)
		}
	}
	return caps
}

// StartHeartbeatMonitor 启动一个后台心跳监控协程，定期检测并标记失联的 Agent 为离线状态。
// 该方法每 30 秒检查一次所有在线 Agent 的最后心跳时间，
// 如果某个 Agent 超过 90 秒未发送心跳，则将其状态更新为 "offline"。
// 该机制确保平台上的 Agent 状态信息始终是准确的。
//
// 参数：
//   - stopCh: 停止信号通道，关闭该通道将终止心跳监控协程
func (s *Service) StartHeartbeatMonitor(stopCh <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			count, err := s.agentRepo.MarkOfflineStale(90 * time.Second)
			if err != nil {
				logger.Errorf("Heartbeat monitor error: %v", err)
			} else if count > 0 {
				logger.Infof("Marked %d stale agents as offline", count)
			}
		}
	}
}

// generateAgentSecret 生成一个随机的 Agent 密钥字符串。
// 使用加密安全的随机数生成器生成 32 字节随机数据，并编码为带 "as-" 前缀的十六进制字符串。
// 生成的密钥格式为 "as-<64个十六进制字符>"，总长度 67 个字符。
//
// 返回值：
//   - string: 生成的 Agent 密钥字符串，格式为 "as-" + 64位十六进制随机串
func generateAgentSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "as-" + hex.EncodeToString(b)
}
