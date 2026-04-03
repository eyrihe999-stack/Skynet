package store

import (
	"time"

	"github.com/skynetplatform/skynet/internal/model"
	"gorm.io/gorm"
)

// InvocationRepo 是调用记录数据仓库，封装了对 invocations 数据表的所有数据库操作。
// 它是 Store 层中负责 Agent 间调用记录持久化的核心组件，
// 提供调用记录的创建、状态更新、查询和列表功能。
// 调用记录（Invocation）记录了一个 Agent 调用另一个 Agent 能力的完整生命周期，
// 包括发起、执行中、完成、失败和取消等状态。
//
// 字段说明：
//   - db: GORM 数据库连接实例，所有数据库操作通过该实例执行
type InvocationRepo struct {
	db *gorm.DB
}

// NewInvocationRepo 创建并返回一个新的调用记录数据仓库实例。
// 这是 InvocationRepo 的构造函数，通过依赖注入的方式接收 GORM 数据库连接。
//
// 参数：
//   - db: GORM 数据库连接实例
//
// 返回值：
//   - *InvocationRepo: 初始化完成的调用记录数据仓库实例
func NewInvocationRepo(db *gorm.DB) *InvocationRepo {
	return &InvocationRepo{db: db}
}

// Create 在数据库中创建一条新的调用记录。
// 该方法在 Agent 发起跨 Agent 调用时被调用，记录调用的初始信息。
//
// 参数：
//   - inv: 要创建的调用记录实体指针，创建成功后其 ID 字段会被自动填充
//
// 返回值：
//   - error: 数据库操作的错误，成功时为 nil
func (r *InvocationRepo) Create(inv *model.Invocation) error {
	return r.db.Create(inv).Error
}

// UpdateStatus 更新指定任务的调用状态及相关信息。
// 该方法在调用完成、失败或取消时被调用，用于更新调用记录的最终状态。
// 当状态为终态（completed、failed、cancelled）时，自动记录完成时间。
//
// 参数：
//   - taskID: 要更新的任务唯一标识符（对应 invocations 表的 task_id 字段）
//   - status: 要设置的新状态，常见值包括 "completed"、"failed"、"cancelled"
//   - latencyMs: 调用延迟时间（毫秒），为 nil 时不更新该字段
//   - errMsg: 错误信息，为空字符串时不更新该字段，调用失败时记录具体的错误描述
//
// 返回值：
//   - error: 数据库更新操作的错误，成功时为 nil
func (r *InvocationRepo) UpdateStatus(taskID string, status string, latencyMs *uint, errMsg string) error {
	updates := map[string]any{"status": status}
	if latencyMs != nil {
		updates["latency_ms"] = *latencyMs
	}
	if errMsg != "" {
		updates["error_message"] = errMsg
	}
	// 当调用进入终态时，自动记录完成时间
	if status == "completed" || status == "failed" || status == "cancelled" {
		now := time.Now()
		updates["completed_at"] = &now
	}
	return r.db.Model(&model.Invocation{}).Where("task_id = ?", taskID).Updates(updates).Error
}

// FindByTaskID 根据任务 ID 查询单条调用记录。
// 该方法用于查询特定调用任务的详细信息，例如检查调用状态或获取调用结果。
//
// 参数：
//   - taskID: 要查询的任务唯一标识符
//
// 返回值：
//   - *model.Invocation: 查询到的调用记录实体，未找到时返回 nil
//   - error: 查询过程中的错误，未找到记录时返回 gorm.ErrRecordNotFound
func (r *InvocationRepo) FindByTaskID(taskID string) (*model.Invocation, error) {
	var inv model.Invocation
	err := r.db.Where("task_id = ?", taskID).First(&inv).Error
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

// InvocationFilter 定义了查询调用记录列表时的过滤和分页条件。
// 该结构体被 InvocationRepo.List 方法使用，支持按调用方、被调用方和发起用户进行筛选。
//
// 字段说明：
//   - CallerAgentID: 调用方 Agent ID 过滤条件，为空时不按调用方过滤
//   - TargetAgentID: 被调用方 Agent ID 过滤条件，为空时不按被调用方过滤
//   - CallerUserID: 发起调用的用户 ID 过滤条件，为 nil 时不按用户过滤
//   - Page: 当前页码，从 1 开始，小于等于 0 时默认为 1
//   - PageSize: 每页记录数，小于等于 0 时默认为 20
type InvocationFilter struct {
	CallerAgentID string
	TargetAgentID string
	CallerUserID  *uint64
	Page          int
	PageSize      int
}

// List 根据过滤条件分页查询调用记录列表。
// 支持按调用方 Agent ID、被调用方 Agent ID 和发起用户 ID 进行筛选。
// 查询结果按创建时间倒序排列，最新的调用记录排在前面。
//
// 参数：
//   - filter: 调用记录查询过滤和分页条件
//
// 返回值：
//   - []model.Invocation: 满足条件的调用记录列表
//   - int64: 满足过滤条件的调用记录总数（不受分页影响，用于前端分页计算）
//   - error: 查询过程中的错误
func (r *InvocationRepo) List(filter InvocationFilter) ([]model.Invocation, int64, error) {
	query := r.db.Model(&model.Invocation{})

	if filter.CallerAgentID != "" {
		query = query.Where("caller_agent_id = ?", filter.CallerAgentID)
	}
	if filter.TargetAgentID != "" {
		query = query.Where("target_agent_id = ?", filter.TargetAgentID)
	}
	if filter.CallerUserID != nil {
		query = query.Where("caller_user_id = ?", *filter.CallerUserID)
	}

	var total int64
	query.Count(&total)

	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}

	var invocations []model.Invocation
	err := query.
		Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).
		Order("created_at DESC").
		Find(&invocations).Error

	return invocations, total, err
}
