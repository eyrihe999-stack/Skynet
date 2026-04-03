-- 用户表补齐字段
ALTER TABLE users
    ADD COLUMN avatar_url VARCHAR(512) DEFAULT NULL AFTER display_name,
    ADD COLUMN auth_provider VARCHAR(50) NOT NULL DEFAULT 'local' AFTER avatar_url;

-- Agent 表补齐字段
ALTER TABLE agents
    ADD COLUMN avatar_url VARCHAR(512) DEFAULT NULL AFTER description;

-- 调用记录表补齐字段
ALTER TABLE invocations
    ADD COLUMN input_ref VARCHAR(512) DEFAULT NULL AFTER skill_name,
    ADD COLUMN output_ref VARCHAR(512) DEFAULT NULL AFTER input_ref,
    ADD COLUMN call_chain JSON DEFAULT NULL AFTER output_ref;

-- 调用记录表扩展状态枚举（添加 input_required）
ALTER TABLE invocations
    MODIFY COLUMN status ENUM('submitted', 'assigned', 'working', 'input_required', 'completed', 'failed', 'cancelled')
    NOT NULL DEFAULT 'submitted';
