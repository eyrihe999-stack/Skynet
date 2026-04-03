// Package config 提供 Skynet 平台 skynetd 守护进程的配置加载功能。
//
// 配置来源按优先级从高到低为：环境变量 > YAML 配置文件 > 内置默认值。
// 支持通过 .env 文件、YAML 配置文件和系统环境变量三种方式设置配置项。
// 本包被 skynetd 的启动入口引用，为服务器监听、数据库连接、JWT 鉴权和日志等
// 模块提供统一的配置来源。
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Config 是 Skynet 平台的顶层配置结构体，聚合了所有子系统的配置。
// 该结构体通过 Load / LoadFrom 函数创建并填充，作为 skynetd 守护进程各模块
// （HTTP 服务器、数据库、鉴权等）的唯一配置来源。
type Config struct {
	// Server 包含 HTTP 服务器相关配置。
	// yaml:"server" 对应 YAML 配置文件中的 server 段。
	Server ServerConfig `yaml:"server"`

	// Database 包含数据库连接相关配置。
	// yaml:"database" 对应 YAML 配置文件中的 database 段。
	Database DatabaseConfig `yaml:"database"`

	// JWT 包含 JSON Web Token 鉴权相关配置。
	// yaml:"jwt" 对应 YAML 配置文件中的 jwt 段。
	JWT JWTConfig `yaml:"jwt"`

	// LogLevel 指定全局日志级别，如 "debug"、"info"、"warn"、"error"。
	// yaml:"log_level" 对应 YAML 配置文件中的 log_level 字段。
	LogLevel string `yaml:"log_level"`

	// Embedding 包含文本向量化（Embedding）服务相关配置。
	// yaml:"embedding" 对应 YAML 配置文件中的 embedding 段。
	Embedding EmbeddingConfig `yaml:"embedding"`
}

// EmbeddingConfig 包含调用外部 Embedding API 所需的配置项。
// 当 BaseURL 和 APIKey 均非空时才启用语义搜索功能。
type EmbeddingConfig struct {
	// BaseURL 是 Embedding API 的基础 URL（如 "https://dashscope.aliyuncs.com/compatible-mode/v1"）。
	// yaml:"base_url" 对应 YAML 中的 embedding.base_url 字段。
	BaseURL string `yaml:"base_url"`

	// APIKey 是调用 Embedding API 所需的鉴权密钥。
	// yaml:"api_key" 对应 YAML 中的 embedding.api_key 字段。
	APIKey string `yaml:"api_key"`

	// Model 是使用的 Embedding 模型名称。默认值为 "text-embedding-v3"。
	// yaml:"model" 对应 YAML 中的 embedding.model 字段。
	Model string `yaml:"model"`
}

// ServerConfig 包含 HTTP 服务器的配置项。
type ServerConfig struct {
	// ListenAddr 是服务器的监听地址，格式为 "host:port" 或 ":port"。
	// 默认值为 ":9090"，表示监听所有网卡的 9090 端口。
	// yaml:"listen_addr" 对应 YAML 中的 server.listen_addr 字段。
	ListenAddr string `yaml:"listen_addr"`
}

// DatabaseConfig 包含 MySQL 数据库连接所需的全部配置项。
// 由 store 层的 Repository 在初始化 GORM 连接时使用。
type DatabaseConfig struct {
	// Host 是数据库服务器地址。默认值为 "127.0.0.1"。
	// yaml:"host" 对应 YAML 中的 database.host 字段。
	Host string `yaml:"host"`

	// Port 是数据库服务端口号。默认值为 "3306"。
	// yaml:"port" 对应 YAML 中的 database.port 字段。
	Port string `yaml:"port"`

	// User 是数据库登录用户名。默认值为 "root"。
	// yaml:"user" 对应 YAML 中的 database.user 字段。
	User string `yaml:"user"`

	// Password 是数据库登录密码。默认值为空字符串。
	// yaml:"password" 对应 YAML 中的 database.password 字段。
	Password string `yaml:"password"`

	// DBName 是目标数据库名称。默认值为 "skynet"。
	// yaml:"db_name" 对应 YAML 中的 database.db_name 字段。
	DBName string `yaml:"db_name"`
}

// DSN 根据当前 DatabaseConfig 的各字段拼接并返回 MySQL Data Source Name 字符串。
// 返回值可直接用于 GORM 的 Open 调用，格式示例：
//
//	"root:pass@tcp(127.0.0.1:3306)/skynet?charset=utf8mb4&parseTime=True&loc=Local"
//
// 连接参数说明：
//   - charset=utf8mb4：使用 UTF-8 全字符集（含 emoji 等四字节字符）
//   - parseTime=True：将 MySQL 的时间类型自动解析为 Go 的 time.Time
//   - loc=Local：使用本机时区
func (d DatabaseConfig) DSN() string {
	return d.User + ":" + d.Password + "@tcp(" + d.Host + ":" + d.Port + ")/" + d.DBName + "?charset=utf8mb4&parseTime=True&loc=Local"
}

// JWTConfig 包含 JWT 鉴权相关配置。
type JWTConfig struct {
	// Secret 是用于签发和验证 JWT Token 的 HMAC 密钥。
	// 默认值为 "change-me-in-production"，生产环境必须通过环境变量或配置文件覆盖。
	// yaml:"secret" 对应 YAML 中的 jwt.secret 字段。
	Secret string `yaml:"secret"`
}

// Load 使用自动解析的配置文件路径加载配置并返回 Config 指针。
// 这是最常用的配置加载入口，等价于 LoadFrom("")。
//
// 配置文件解析优先级（取第一个存在的文件）：
//  1. config/config.{ENV}.yaml 或 config.{ENV}.yaml（ENV 环境变量指定环境名）
//  2. config/config.yaml 或 config.yaml
//
// 在文件加载之后，环境变量会覆盖 YAML 中的同名配置。
//
// 返回值：填充完毕的 *Config，包含默认值、YAML 文件值和环境变量覆盖值。
func Load() *Config {
	return LoadFrom("")
}

// LoadFrom 从指定路径加载配置文件并返回 Config 指针。
//
// 参数：
//   - path：YAML 配置文件的路径。若为空字符串，则自动通过 resolveConfigPath 解析。
//
// 加载流程：
//  1. 尝试加载项目根目录下的 .env 文件（若存在），将其中的键值对注入环境变量。
//  2. 使用内置默认值初始化 Config 结构体。
//  3. 读取并解析 YAML 配置文件，用文件中的值覆盖默认值。
//  4. 逐项检查环境变量，用环境变量的值覆盖 YAML / 默认值。
//
// 返回值：填充完毕的 *Config。
func LoadFrom(path string) *Config {
	_ = godotenv.Load()

	cfg := &Config{
		Server:   ServerConfig{ListenAddr: ":9090"},
		Database: DatabaseConfig{Host: "127.0.0.1", Port: "3306", User: "root", DBName: "skynet"},
		JWT:      JWTConfig{Secret: "change-me-in-production"},
		LogLevel: "info",
		Embedding: EmbeddingConfig{
			Model: "text-embedding-v3",
		},
	}

	// 解析配置文件路径：若调用方未指定则自动查找
	if path == "" {
		path = resolveConfigPath()
	}

	if path != "" {
		if data, err := os.ReadFile(path); err == nil {
			yaml.Unmarshal(data, cfg)
			fmt.Printf("Loaded config: %s\n", path)
		}
	}

	// 环境变量覆盖 YAML 中的配置值
	overrideFromEnv(&cfg.Server.ListenAddr, "LISTEN_ADDR")
	overrideFromEnv(&cfg.Database.Host, "DB_HOST")
	overrideFromEnv(&cfg.Database.Port, "DB_PORT")
	overrideFromEnv(&cfg.Database.User, "DB_USER")
	overrideFromEnv(&cfg.Database.Password, "DB_PASSWORD")
	overrideFromEnv(&cfg.Database.DBName, "DB_NAME")
	overrideFromEnv(&cfg.JWT.Secret, "JWT_SECRET")
	overrideFromEnv(&cfg.LogLevel, "LOG_LEVEL")
	overrideFromEnv(&cfg.Embedding.BaseURL, "EMBEDDING_BASE_URL")
	overrideFromEnv(&cfg.Embedding.APIKey, "EMBEDDING_API_KEY")
	overrideFromEnv(&cfg.Embedding.Model, "EMBEDDING_MODEL")

	return cfg
}

// resolveConfigPath 按照优先级自动查找并返回配置文件路径。
//
// 查找规则：
//  1. 若设置了 ENV 环境变量（如 "dev"、"prod"），则依次查找
//     config/config.{ENV}.yaml 和 config.{ENV}.yaml。
//  2. 若上一步未找到，则依次查找 config/config.yaml 和 config.yaml。
//  3. 若所有候选文件均不存在，返回空字符串，表示无外部配置文件可用。
//
// 返回值：第一个存在的配置文件路径，或空字符串。
func resolveConfigPath() string {
	// 1. 按环境名查找：config/config.dev.yaml、config/config.prod.yaml 等
	if env := os.Getenv("ENV"); env != "" {
		candidates := []string{
			"config/config." + env + ".yaml",
			"config." + env + ".yaml",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}

	// 2. 查找默认配置文件
	for _, c := range []string{"config/config.yaml", "config.yaml"} {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	return ""
}

// overrideFromEnv 检查指定环境变量是否存在，若存在则用其值覆盖 target 指向的字符串。
//
// 参数：
//   - target：指向待覆盖配置项的指针。
//   - envKey：要检查的环境变量名称。
//
// 该函数不返回任何值；若环境变量未设置或值为空字符串，则 target 保持不变。
func overrideFromEnv(target *string, envKey string) {
	if v := os.Getenv(envKey); v != "" {
		*target = v
	}
}
