package store

import (
	"github.com/eyrihe999-stack/Skynet/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CapabilityRepo 是能力数据仓库，封装了对 capabilities 数据表的所有数据库操作。
// 它是 Store 层中负责 Agent 能力（Capability）持久化的核心组件，
// 提供批量创建/更新、查询、全文搜索和调用统计等功能。
// 能力是 Agent 对外暴露的服务接口声明，是 Agent 发现和调用机制的基础数据。
//
// 字段说明：
//   - db: GORM 数据库连接实例，所有数据库操作通过该实例执行
type CapabilityRepo struct {
	db *gorm.DB
}

// NewCapabilityRepo 创建并返回一个新的能力数据仓库实例。
// 这是 CapabilityRepo 的构造函数，通过依赖注入的方式接收 GORM 数据库连接。
//
// 参数：
//   - db: GORM 数据库连接实例
//
// 返回值：
//   - *CapabilityRepo: 初始化完成的能力数据仓库实例
func NewCapabilityRepo(db *gorm.DB) *CapabilityRepo {
	return &CapabilityRepo{db: db}
}

// BulkUpsert 在一个事务中批量创建或更新指定 Agent 的能力列表。
// 该方法实现了"声明式同步"语义：以传入的能力列表为基准，
// 删除数据库中已不存在的旧能力，并对保留的能力执行 Upsert 操作。
// 整个操作在单个数据库事务中完成，保证原子性。
//
// 处理逻辑：
//  1. 收集本次传入的所有能力名称
//  2. 删除数据库中该 Agent 下不在本次列表中的旧能力记录
//  3. 逐条对本次列表中的能力执行 Upsert（INSERT ... ON CONFLICT DO UPDATE）
//
// 参数：
//   - agentID: 能力所属的 Agent 唯一标识符
//   - caps: 要同步的能力列表，列表中的能力会被创建或更新，不在列表中的旧能力会被删除
//
// 返回值：
//   - error: 事务执行过程中的错误，任何步骤失败都会导致整个事务回滚
func (r *CapabilityRepo) BulkUpsert(agentID string, caps []model.Capability) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 收集本次要保留的能力名称列表
		keepNames := make([]string, len(caps))
		for i, c := range caps {
			keepNames[i] = c.Name
		}
		// 删除数据库中该 Agent 下不在保留列表中的旧能力
		if len(keepNames) > 0 {
			tx.Where("agent_id = ? AND name NOT IN ?", agentID, keepNames).
				Delete(&model.Capability{})
		} else {
			// 如果新列表为空，则删除该 Agent 的所有能力
			tx.Where("agent_id = ?", agentID).Delete(&model.Capability{})
		}

		// 逐条执行 Upsert，以 (agent_id, name) 为冲突判断键
		for i := range caps {
			caps[i].AgentID = agentID
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "agent_id"}, {Name: "name"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"display_name", "description", "category", "tags",
					"input_schema", "output_schema", "visibility",
					"approval_mode", "multi_turn", "async",
					"estimated_latency_ms", "updated_at",
				}),
			}).Create(&caps[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// FindByAgentAndName 根据 Agent ID 和 Skill 名称查询单个能力记录。
// 该方法用于调用前的权限校验，通过 (agent_id, name) 联合唯一索引精确定位目标 Skill。
//
// 参数：
//   - agentID: 能力所属 Agent 的唯一标识符
//   - name: 能力的程序化名称
//
// 返回值：
//   - *model.Capability: 查询到的能力记录，未找到时返回 nil
//   - error: 查询过程中的错误，未找到记录时返回 gorm.ErrRecordNotFound
func (r *CapabilityRepo) FindByAgentAndName(agentID, name string) (*model.Capability, error) {
	var cap model.Capability
	err := r.db.Where("agent_id = ? AND name = ?", agentID, name).First(&cap).Error
	if err != nil {
		return nil, err
	}
	return &cap, nil
}

// FindByAgentID 查询指定 Agent 的所有能力记录。
// 该方法不做可见性或状态过滤，返回该 Agent 下的完整能力列表。
//
// 参数：
//   - agentID: 要查询能力的 Agent 唯一标识符
//
// 返回值：
//   - []model.Capability: 该 Agent 的能力列表，无记录时返回空切片
//   - error: 查询过程中的错误
func (r *CapabilityRepo) FindByAgentID(agentID string) ([]model.Capability, error) {
	var caps []model.Capability
	err := r.db.Where("agent_id = ?", agentID).Find(&caps).Error
	return caps, err
}

// CapabilityFilter 定义了搜索能力时的过滤和分页条件。
// 该结构体被 CapabilityRepo.Search 方法使用，支持全文搜索和按分类筛选。
//
// 字段说明：
//   - Query: 全文搜索关键词，使用 MySQL FULLTEXT 索引在 display_name 和 description 上进行布尔模式搜索，为空时不执行全文搜索
//   - Category: 能力分类过滤条件，为空时不按分类过滤
//   - Page: 当前页码，从 1 开始，小于等于 0 时默认为 1
//   - PageSize: 每页记录数，小于等于 0 时默认为 20
type CapabilityFilter struct {
	Query    string
	Category string
	Page     int
	PageSize int
}

// Search 根据过滤条件搜索可用的公开能力，支持全文搜索和分类过滤。
// 该方法仅返回可见性为 "public" 且所属 Agent 状态为 "online" 的能力，
// 确保搜索结果中的能力都是当前可用的。
// 全文搜索使用 MySQL 的 FULLTEXT 索引，在 display_name 和 description 字段上执行布尔模式匹配。
//
// 参数：
//   - filter: 搜索过滤和分页条件
//
// 返回值：
//   - []model.Capability: 满足条件的能力列表
//   - int64: 满足过滤条件的能力总数（不受分页影响，用于前端分页计算）
//   - error: 搜索过程中的错误
func (r *CapabilityRepo) Search(filter CapabilityFilter) ([]model.Capability, int64, error) {
	query := r.db.Model(&model.Capability{}).
		Where("visibility = 'public'").
		Joins("JOIN agents ON agents.agent_id = capabilities.agent_id AND agents.status = 'online'")

	if filter.Query != "" {
		query = query.Where(
			"MATCH(capabilities.display_name, capabilities.description) AGAINST(? IN BOOLEAN MODE)",
			filter.Query,
		)
	}
	if filter.Category != "" {
		query = query.Where("capabilities.category = ?", filter.Category)
	}

	var total int64
	query.Count(&total)

	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}

	var caps []model.Capability
	err := query.
		Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).
		Find(&caps).Error

	return caps, total, err
}

// FindByID 根据主键 ID 查询单个能力记录。
//
// 参数：
//   - id: 能力记录的自增主键
//
// 返回值：
//   - *model.Capability: 查询到的能力记录，未找到时返回 nil
//   - error: 查询过程中的错误，未找到记录时返回 gorm.ErrRecordNotFound
func (r *CapabilityRepo) FindByID(id uint64) (*model.Capability, error) {
	var cap model.Capability
	err := r.db.First(&cap, id).Error
	if err != nil {
		return nil, err
	}
	return &cap, nil
}

// IncrementCallCount 递增指定能力的调用统计计数器。
// 该方法在每次能力被调用后执行，用于记录调用次数、累计延迟和成功次数，
// 为后续的能力质量评估和排序提供数据支撑。
// 所有计数器使用 SQL 表达式原子递增，避免并发更新时的数据丢失。
//
// 参数：
//   - agentID: 被调用能力所属的 Agent 唯一标识符
//   - skillName: 被调用能力的名称（对应 capabilities 表的 name 字段）
//   - latencyMs: 本次调用的延迟时间（毫秒），累加到 total_latency_ms 字段
//   - success: 本次调用是否成功，为 true 时 success_count 计数器也会递增
//
// 返回值：
//   - error: 数据库更新操作的错误，成功时为 nil
func (r *CapabilityRepo) IncrementCallCount(agentID, skillName string, latencyMs uint, success bool) error {
	updates := map[string]any{
		"call_count":       gorm.Expr("call_count + 1"),
		"total_latency_ms": gorm.Expr("total_latency_ms + ?", latencyMs),
	}
	if success {
		updates["success_count"] = gorm.Expr("success_count + 1")
	}

	return r.db.Model(&model.Capability{}).
		Where("agent_id = ? AND name = ?", agentID, skillName).
		Updates(updates).Error
}
