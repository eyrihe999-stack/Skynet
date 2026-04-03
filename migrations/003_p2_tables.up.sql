-- 权限规则表
CREATE TABLE IF NOT EXISTS permission_rules (
    id                  BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    agent_id            VARCHAR(100) NOT NULL,
    skill_name          VARCHAR(100) DEFAULT NULL,
    caller_type         ENUM('user', 'agent', 'any') NOT NULL DEFAULT 'any',
    caller_id           VARCHAR(100) DEFAULT NULL,
    action              ENUM('allow', 'deny') NOT NULL DEFAULT 'allow',
    approval_mode       ENUM('auto', 'manual') NOT NULL DEFAULT 'auto',
    rate_limit_max      INT UNSIGNED DEFAULT NULL,
    rate_limit_window   VARCHAR(10) DEFAULT NULL,
    priority            INT NOT NULL DEFAULT 0,
    created_at          DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    FOREIGN KEY (agent_id) REFERENCES agents(agent_id) ON DELETE CASCADE,
    INDEX idx_agent_skill (agent_id, skill_name),
    INDEX idx_caller (caller_type, caller_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 审批队列表
CREATE TABLE IF NOT EXISTS approval_queue (
    id                  BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    invocation_id       BIGINT UNSIGNED NOT NULL,
    owner_id            BIGINT UNSIGNED NOT NULL,
    status              ENUM('pending', 'approved', 'denied', 'expired') NOT NULL DEFAULT 'pending',
    decided_at          DATETIME(3) DEFAULT NULL,
    expires_at          DATETIME(3) NOT NULL,
    created_at          DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    INDEX idx_owner_status (owner_id, status),
    INDEX idx_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 多轮对话消息表
CREATE TABLE IF NOT EXISTS task_messages (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    task_id         VARCHAR(36) NOT NULL,
    direction       ENUM('to_agent', 'from_agent') NOT NULL,
    message_type    ENUM('input', 'output', 'question', 'reply', 'progress') NOT NULL,
    payload_ref     VARCHAR(512) DEFAULT NULL,
    created_at      DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    INDEX idx_task (task_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
