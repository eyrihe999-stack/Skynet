CREATE TABLE IF NOT EXISTS users (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    email           VARCHAR(255) NOT NULL UNIQUE,
    display_name    VARCHAR(100) NOT NULL,
    password_hash   VARCHAR(255) DEFAULT NULL,
    api_key_hash    VARCHAR(255) DEFAULT NULL,
    status          ENUM('active', 'suspended', 'deleted') NOT NULL DEFAULT 'active',
    created_at      DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at      DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS agents (
    id                  BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    agent_id            VARCHAR(100) NOT NULL UNIQUE,
    owner_id            BIGINT UNSIGNED NOT NULL,
    display_name        VARCHAR(200) NOT NULL,
    description         TEXT,
    connection_mode     ENUM('tunnel', 'direct') NOT NULL DEFAULT 'tunnel',
    endpoint_url        VARCHAR(512) DEFAULT NULL,
    agent_secret_hash   VARCHAR(255) NOT NULL,
    data_policy         JSON DEFAULT NULL,
    status              ENUM('online', 'offline', 'removed') NOT NULL DEFAULT 'offline',
    last_heartbeat_at   DATETIME(3) DEFAULT NULL,
    framework_version   VARCHAR(50) DEFAULT NULL,
    version             VARCHAR(50) DEFAULT '1.0.0',
    created_at          DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at          DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    FOREIGN KEY (owner_id) REFERENCES users(id),
    INDEX idx_owner (owner_id),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS capabilities (
    id                  BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    agent_id            VARCHAR(100) NOT NULL,
    name                VARCHAR(100) NOT NULL,
    display_name        VARCHAR(200) NOT NULL,
    description         TEXT,
    category            VARCHAR(50) DEFAULT 'general',
    tags                JSON DEFAULT NULL,
    input_schema        JSON NOT NULL,
    output_schema       JSON DEFAULT NULL,
    visibility          ENUM('public', 'restricted', 'private') NOT NULL DEFAULT 'public',
    approval_mode       ENUM('auto', 'manual') NOT NULL DEFAULT 'auto',
    multi_turn          TINYINT(1) NOT NULL DEFAULT 0,
    estimated_latency_ms INT UNSIGNED DEFAULT NULL,
    call_count          BIGINT UNSIGNED NOT NULL DEFAULT 0,
    success_count       BIGINT UNSIGNED NOT NULL DEFAULT 0,
    total_latency_ms    BIGINT UNSIGNED NOT NULL DEFAULT 0,
    created_at          DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at          DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_agent_skill (agent_id, name),
    FOREIGN KEY (agent_id) REFERENCES agents(agent_id) ON DELETE CASCADE,
    INDEX idx_category (category),
    INDEX idx_visibility (visibility),
    FULLTEXT INDEX ft_search (display_name, description)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS invocations (
    id                  BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    task_id             VARCHAR(36) NOT NULL UNIQUE,
    caller_agent_id     VARCHAR(100) DEFAULT NULL,
    caller_user_id      BIGINT UNSIGNED DEFAULT NULL,
    target_agent_id     VARCHAR(100) NOT NULL,
    skill_name          VARCHAR(100) NOT NULL,
    status              ENUM('submitted', 'assigned', 'working', 'completed', 'failed', 'cancelled')
                        NOT NULL DEFAULT 'submitted',
    mode                ENUM('sync', 'async') NOT NULL DEFAULT 'sync',
    error_message       TEXT DEFAULT NULL,
    latency_ms          INT UNSIGNED DEFAULT NULL,
    created_at          DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    completed_at        DATETIME(3) DEFAULT NULL,
    INDEX idx_caller_agent (caller_agent_id, created_at),
    INDEX idx_caller_user (caller_user_id, created_at),
    INDEX idx_target (target_agent_id, created_at),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
