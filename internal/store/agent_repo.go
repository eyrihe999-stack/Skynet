// Package store 提供 Skynet 平台的数据持久化层，采用 Repository 模式封装数据库访问。
// 该包包含 Agent、Capability 和 Invocation 三个数据仓库，
// 基于 GORM ORM 框架实现对 MySQL 数据库的 CRUD 操作。
// Store 层是 Registry 服务层的下层依赖，负责所有数据库交互的细节。
package store

import (
	"time"

	"github.com/eyrihe999-stack/Skynet/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AgentRepo 是 Agent 数据仓库，封装了对 agents 数据表的所有数据库操作。
// 它是 Store 层中负责 Agent 实体持久化的核心组件，提供创建、更新、查询、
// 状态管理和心跳维护等功能。Registry 服务层通过该仓库完成 Agent 相关的数据库交互。
//
// 字段说明：
//   - db: GORM 数据库连接实例，所有数据库操作通过该实例执行
type AgentRepo struct {
	db *gorm.DB
}

// NewAgentRepo 创建并返回一个新的 Agent 数据仓库实例。
// 这是 AgentRepo 的构造函数，通过依赖注入的方式接收 GORM 数据库连接。
//
// 参数：
//   - db: GORM 数据库连接实例
//
// 返回值：
//   - *AgentRepo: 初始化完成的 Agent 数据仓库实例
func NewAgentRepo(db *gorm.DB) *AgentRepo {
	return &AgentRepo{db: db}
}

// Create 在数据库中创建一条新的 Agent 记录。
// 该方法直接执行 INSERT 操作，如果 agent_id 已存在会返回唯一约束冲突错误。
// 对于需要支持重复注册的场景，请使用 Upsert 方法。
//
// 参数：
//   - agent: 要创建的 Agent 实体指针，创建成功后其 ID 字段会被自动填充
//
// 返回值：
//   - error: 数据库操作的错误，成功时为 nil
func (r *AgentRepo) Create(agent *model.Agent) error {
	return r.db.Create(agent).Error
}

// Upsert 创建或更新一条 Agent 记录（INSERT ... ON CONFLICT DO UPDATE）。
// 以 agent_id 作为冲突判断列：若记录不存在则执行插入；若已存在则更新指定字段。
// 该方法是 Agent 注册流程的核心数据操作，支持 Agent 的首次注册和信息更新。
//
// 更新的字段包括：display_name、description、connection_mode、endpoint_url、
// data_policy、framework_version、version、status、last_heartbeat_at、updated_at。
//
// 参数：
//   - agent: 要创建或更新的 Agent 实体指针
//
// 返回值：
//   - error: 数据库操作的错误，成功时为 nil
func (r *AgentRepo) Upsert(agent *model.Agent) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "agent_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"display_name", "description", "connection_mode",
			"endpoint_url", "data_policy", "framework_version",
			"version", "status", "last_heartbeat_at", "updated_at",
		}),
	}).Create(agent).Error
}

// FindByAgentID 根据 Agent ID 查询单个 Agent 记录，并预加载其关联的能力列表。
// 该方法用于获取 Agent 的完整信息，包括其声明的所有 Capability。
//
// 参数：
//   - agentID: 要查询的 Agent 唯一标识符
//
// 返回值：
//   - *model.Agent: 查询到的 Agent 实体（含预加载的 Capabilities 关联），未找到时返回 nil
//   - error: 查询过程中的错误，未找到记录时返回 gorm.ErrRecordNotFound
func (r *AgentRepo) FindByAgentID(agentID string) (*model.Agent, error) {
	var agent model.Agent
	err := r.db.Preload("Capabilities").Where("agent_id = ?", agentID).First(&agent).Error
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

// UpdateStatus 更新指定 Agent 的状态字段。
// 常见的状态值包括 "online"（在线）、"offline"（离线）、"removed"（已删除）。
// 该方法被注销、删除等业务流程调用。
//
// 参数：
//   - agentID: 要更新状态的 Agent 唯一标识符
//   - status: 要设置的新状态值
//
// 返回值：
//   - error: 数据库更新操作的错误，成功时为 nil
func (r *AgentRepo) UpdateStatus(agentID string, status string) error {
	return r.db.Model(&model.Agent{}).Where("agent_id = ?", agentID).
		Update("status", status).Error
}

// UpdateHeartbeat 更新指定 Agent 的心跳时间戳，并将其状态设置为在线。
// 该方法在收到 Agent 心跳请求时调用，同时更新 last_heartbeat_at 和 status 两个字段。
// 即使 Agent 之前因超时被标记为离线，收到心跳后也会被重新标记为在线。
//
// 参数：
//   - agentID: 发送心跳的 Agent 唯一标识符
//
// 返回值：
//   - error: 数据库更新操作的错误，成功时为 nil
func (r *AgentRepo) UpdateHeartbeat(agentID string) error {
	now := time.Now()
	return r.db.Model(&model.Agent{}).Where("agent_id = ?", agentID).
		Updates(map[string]any{
			"last_heartbeat_at": now,
			"status":           "online",
		}).Error
}

// AgentFilter 定义了查询 Agent 列表时的过滤和分页条件。
// 该结构体被 AgentRepo.List 方法使用，支持按状态和所有者 ID 进行筛选。
//
// 字段说明：
//   - Status: Agent 状态过滤条件，为空时不按状态过滤
//   - OwnerID: 所有者用户 ID 过滤条件，为 nil 时不按所有者过滤
//   - Page: 当前页码，从 1 开始，小于等于 0 时默认为 1
//   - PageSize: 每页记录数，小于等于 0 时默认为 20
type AgentFilter struct {
	Status   string
	OwnerID  *uint64
	Page     int
	PageSize int
}

// List 根据过滤条件分页查询 Agent 列表。
// 自动排除状态为 "removed" 的已删除 Agent，支持按状态和所有者 ID 过滤。
// 查询结果预加载 Capabilities 关联，按创建时间倒序排列。
//
// 参数：
//   - filter: Agent 查询过滤和分页条件
//
// 返回值：
//   - []model.Agent: 满足条件的 Agent 列表（含预加载的 Capabilities）
//   - int64: 满足过滤条件的 Agent 总数（不受分页影响，用于前端分页计算）
//   - error: 查询过程中的错误
func (r *AgentRepo) List(filter AgentFilter) ([]model.Agent, int64, error) {
	query := r.db.Model(&model.Agent{}).Where("status != 'removed'")

	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.OwnerID != nil {
		query = query.Where("owner_id = ?", *filter.OwnerID)
	}

	var total int64
	query.Count(&total)

	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}

	var agents []model.Agent
	err := query.Preload("Capabilities").
		Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).
		Order("created_at DESC").
		Find(&agents).Error

	return agents, total, err
}

// MarkOfflineStale 将超过心跳阈值的在线 Agent 批量标记为离线状态。
// 该方法由心跳监控协程定期调用，是 Agent 存活检测机制的核心数据操作。
// 只影响当前状态为 "online" 且最后心跳时间早于阈值的 Agent。
//
// 参数：
//   - threshold: 心跳超时阈值，超过该时间未心跳的 Agent 将被标记为离线
//
// 返回值：
//   - int64: 被标记为离线的 Agent 数量
//   - error: 数据库更新操作的错误，成功时为 nil
func (r *AgentRepo) MarkOfflineStale(threshold time.Duration) (int64, error) {
	cutoff := time.Now().Add(-threshold)
	result := r.db.Model(&model.Agent{}).
		Where("status = 'online' AND last_heartbeat_at < ?", cutoff).
		Update("status", "offline")
	return result.RowsAffected, result.Error
}

// Delete 逻辑删除指定的 Agent，将其状态设置为 "removed"。
// 该方法不执行物理删除，而是通过状态标记实现软删除。
// 被标记为 "removed" 的 Agent 将不会出现在 List 查询结果中。
//
// 参数：
//   - agentID: 要删除的 Agent 唯一标识符
//
// 返回值：
//   - error: 数据库更新操作的错误，成功时为 nil
func (r *AgentRepo) Delete(agentID string) error {
	return r.db.Model(&model.Agent{}).Where("agent_id = ?", agentID).
		Update("status", "removed").Error
}
