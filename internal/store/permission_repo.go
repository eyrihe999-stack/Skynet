package store

import (
	"github.com/eyrihe999-stack/Skynet/internal/model"
	"gorm.io/gorm"
)

// PermissionRepo 是权限规则数据仓库，封装了对 permission_rules 数据表的所有数据库操作。
// 它负责细粒度权限规则的增删查，供 Gateway 权限校验和 API 管理端点使用。
//
// 字段说明：
//   - db: GORM 数据库连接实例，所有数据库操作通过该实例执行
type PermissionRepo struct {
	db *gorm.DB
}

// NewPermissionRepo 创建并返回一个新的权限规则数据仓库实例。
//
// 参数：
//   - db: GORM 数据库连接实例
//
// 返回值：
//   - *PermissionRepo: 初始化完成的权限规则数据仓库实例
func NewPermissionRepo(db *gorm.DB) *PermissionRepo {
	return &PermissionRepo{db: db}
}

// FindRules 查询匹配的权限规则，按 priority 降序排列。
// 匹配条件：agent_id 匹配 AND (skill_name 匹配或为 NULL 表示全局规则)
//
// 参数：
//   - agentID: 目标 Agent 的唯一标识符
//   - skillName: 目标技能名称，同时匹配精确规则和全局规则（skill_name IS NULL）
//
// 返回值：
//   - []model.PermissionRule: 匹配的权限规则列表，按优先级降序、ID 升序排列
//   - error: 数据库查询错误，成功时为 nil
func (r *PermissionRepo) FindRules(agentID, skillName string) ([]model.PermissionRule, error) {
	var rules []model.PermissionRule
	err := r.db.Where("agent_id = ? AND (skill_name = ? OR skill_name IS NULL)", agentID, skillName).
		Order("priority DESC, id ASC").
		Find(&rules).Error
	return rules, err
}

// Create 创建一条权限规则。
//
// 参数：
//   - rule: 要创建的权限规则实体指针，创建成功后其 ID 字段会被自动填充
//
// 返回值：
//   - error: 数据库操作的错误，成功时为 nil
func (r *PermissionRepo) Create(rule *model.PermissionRule) error {
	return r.db.Create(rule).Error
}

// Update 更新一条权限规则。
//
// 参数：
//   - rule: 要更新的权限规则实体指针，必须包含有效的 ID
//
// 返回值：
//   - error: 数据库操作的错误，成功时为 nil
func (r *PermissionRepo) Update(rule *model.PermissionRule) error {
	return r.db.Save(rule).Error
}

// Delete 删除一条权限规则。
//
// 参数：
//   - id: 要删除的权限规则 ID
//
// 返回值：
//   - error: 数据库操作的错误，成功时为 nil
func (r *PermissionRepo) Delete(id uint64) error {
	return r.db.Delete(&model.PermissionRule{}, id).Error
}

// FindByAgent 查询某个 Agent 的所有权限规则，按优先级降序排列。
//
// 参数：
//   - agentID: 目标 Agent 的唯一标识符
//
// 返回值：
//   - []model.PermissionRule: 该 Agent 的所有权限规则列表
//   - error: 数据库查询错误，成功时为 nil
func (r *PermissionRepo) FindByAgent(agentID string) ([]model.PermissionRule, error) {
	var rules []model.PermissionRule
	err := r.db.Where("agent_id = ?", agentID).Order("priority DESC").Find(&rules).Error
	return rules, err
}

// FindByID 根据 ID 查询单条权限规则。
//
// 参数：
//   - id: 权限规则 ID
//
// 返回值：
//   - *model.PermissionRule: 查询到的权限规则，未找到时返回 nil
//   - error: 查询错误，未找到记录时返回 gorm.ErrRecordNotFound
func (r *PermissionRepo) FindByID(id uint64) (*model.PermissionRule, error) {
	var rule model.PermissionRule
	err := r.db.First(&rule, id).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}
