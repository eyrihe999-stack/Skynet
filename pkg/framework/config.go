package framework

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// AgentConfig 是 skynet.yaml 配置文件的顶层结构体。
//
// 该结构体定义了 Agent 运行所需的全部配置，包括 Agent 基本信息、
// 网络连接配置、本地服务器配置以及 Skill 默认值。
// 配置文件支持环境变量展开和多环境配置文件（如 skynet.dev.yaml）。
//
// 字段说明：
//   - Agent: Agent 的基本身份信息（ID、名称、描述、版本）
//   - Network: 网络连接配置（Gateway 注册地址、API 密钥）
//   - Server: 本地开发服务器配置（监听端口）
//   - Defaults: Skill 的默认配置（可见性、审批模式、速率限制）
type AgentConfig struct {
	Agent    AgentSection    `yaml:"agent"`
	Network  NetworkSection  `yaml:"network"`
	Server   ServerSection   `yaml:"server"`
	Defaults DefaultsSection `yaml:"defaults"`
}

// AgentSection 定义 Agent 的基本身份信息配置段。
//
// 字段说明：
//   - ID: Agent 的全局唯一标识符，用于在 Skynet 网络中标识该 Agent
//   - DisplayName: Agent 的显示名称，用于 UI 展示
//   - Description: Agent 的功能描述
//   - Version: Agent 的版本号，默认为 "1.0.0"
type AgentSection struct {
	ID          string `yaml:"id"`
	DisplayName string `yaml:"display_name"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
}

// NetworkSection 定义 Agent 的网络连接配置段。
//
// 当 Registry 和 APIKey 都已配置时，Agent 以生产模式运行，
// 通过 WebSocket 反向通道连接 Gateway；否则以开发模式运行。
//
// 字段说明：
//   - Registry: Skynet Gateway 的注册地址（HTTP/HTTPS URL），支持环境变量展开
//   - APIKey: Agent 的 API 密钥，用于身份认证，支持环境变量展开（如 ${SKYNET_API_KEY}）
type NetworkSection struct {
	Registry string `yaml:"registry"`
	APIKey   string `yaml:"api_key"`
}

// ServerSection 定义本地开发服务器的配置段。
//
// 字段说明：
//   - Port: 本地 HTTP 服务器监听端口，默认为 9100
type ServerSection struct {
	Port int `yaml:"port"`
}

// DefaultsSection 定义 Skill 的默认配置段。
//
// 当 Skill 未显式指定可见性或审批模式时，使用此处的默认值。
//
// 字段说明：
//   - Visibility: Skill 的默认可见性，默认为 "public"
//   - ApprovalMode: Skill 的默认审批模式，默认为 "auto"
//   - RateLimit: 速率限制配置
type DefaultsSection struct {
	Visibility   string    `yaml:"visibility"`
	ApprovalMode string    `yaml:"approval_mode"`
	RateLimit    RateLimit `yaml:"rate_limit"`
}

// RateLimit 定义速率限制配置。
//
// 字段说明：
//   - Max: 在时间窗口内允许的最大请求数
//   - Window: 时间窗口大小（如 "1m"、"1h"）
type RateLimit struct {
	Max    int    `yaml:"max"`
	Window string `yaml:"window"`
}

// LoadConfig 从当前目录加载 skynet.yaml 配置文件，支持环境特定的配置文件。
//
// 配置文件搜索优先级（第一个匹配的生效）：
//  1. skynet.{ENV}.yaml — 根据 ENV 环境变量选择，如 skynet.dev.yaml、skynet.prod.yaml
//  2. skynet.yaml — 默认配置文件
//
// 返回值：
//   - *AgentConfig: 解析后的 Agent 配置
//   - error: 配置文件未找到或解析失败时返回错误
func LoadConfig() (*AgentConfig, error) {
	path := resolveAgentConfigPath()
	if path == "" {
		return nil, fmt.Errorf("skynet.yaml not found in current directory")
	}
	fmt.Printf("Loaded config: %s\n", path)
	return LoadConfigFrom(path)
}

// LoadConfigFrom 从指定路径读取并解析 skynet.yaml 配置文件。
//
// 该函数会执行以下处理：
//  1. 读取 YAML 文件并反序列化为 AgentConfig 结构体
//  2. 对敏感字段（APIKey、Registry）执行环境变量展开
//  3. 为未设置的字段填充默认值（Version="1.0.0"、Port=9100、Visibility="public"、ApprovalMode="auto"）
//
// 参数：
//   - path: skynet.yaml 配置文件的文件路径
//
// 返回值：
//   - *AgentConfig: 解析并应用默认值后的 Agent 配置
//   - error: 文件读取失败或 YAML 解析失败时返回错误
func LoadConfigFrom(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &AgentConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// 展开敏感字段中的环境变量（如 $SKYNET_API_KEY 或 ${SKYNET_API_KEY}）
	if cfg.Network.APIKey != "" {
		cfg.Network.APIKey = os.ExpandEnv(cfg.Network.APIKey)
	}
	if cfg.Network.Registry != "" {
		cfg.Network.Registry = os.ExpandEnv(cfg.Network.Registry)
	}

	// 为未设置的字段填充默认值
	if cfg.Agent.Version == "" {
		cfg.Agent.Version = "1.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 9100
	}
	if cfg.Defaults.Visibility == "" {
		cfg.Defaults.Visibility = "public"
	}
	if cfg.Defaults.ApprovalMode == "" {
		cfg.Defaults.ApprovalMode = "auto"
	}

	return cfg, nil
}

// IsDevMode 判断当前 Agent 是否应以本地开发模式运行。
//
// 当以下任一条件满足时，返回 true（进入开发模式）：
//   - 未配置 Registry（Gateway 注册地址为空）
//   - 未配置 APIKey（API 密钥为空）
//
// 开发模式下，Agent 会启动本地 HTTP 服务器而非连接 Skynet 网络。
//
// 返回值：
//   - bool: true 表示开发模式，false 表示生产模式
func (c *AgentConfig) IsDevMode() bool {
	return c.Network.Registry == "" || c.Network.APIKey == ""
}

// resolveAgentConfigPath 按优先级搜索配置文件路径。
//
// 搜索顺序：
//  1. 环境特定配置：skynet.{ENV}.yaml（如 ENV=dev 时查找 skynet.dev.yaml）
//  2. 默认配置：skynet.yaml
//
// 返回值：
//   - string: 找到的配置文件路径，未找到时返回空字符串
func resolveAgentConfigPath() string {
	// 1. 环境特定配置：skynet.dev.yaml、skynet.prod.yaml 等
	if env := os.Getenv("ENV"); env != "" {
		path := "skynet." + env + ".yaml"
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// 2. 默认配置
	if _, err := os.Stat("skynet.yaml"); err == nil {
		return "skynet.yaml"
	}

	return ""
}
