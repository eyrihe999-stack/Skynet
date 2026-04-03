// Package authz 实现 Skynet 平台的认证授权模块。
//
// 该模块提供用户注册、登录、API Key 验证和 JWT 验证等核心认证能力。
// 用户通过 API Key 或 JWT 访问平台 API，Agent 通过 API Key 建立 WebSocket 隧道。
// 本包包含认证服务层（Service）、HTTP 处理层（Handler）以及 Gin 中间件（Middleware）。
package authz

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/eyrihe999-stack/Skynet/internal/model"
	"github.com/eyrihe999-stack/Skynet-sdk/logger"
	"gorm.io/gorm"
)

var (
	// ErrUserNotFound 表示在数据库中未找到对应的用户，
	// 通常在登录或 JWT 验证时因用户不存在或状态非活跃而返回。
	ErrUserNotFound = errors.New("user not found")

	// ErrInvalidPassword 表示用户提供的密码与数据库中存储的哈希不匹配，
	// 在登录流程中密码校验失败时返回。
	ErrInvalidPassword = errors.New("invalid password")

	// ErrEmailExists 表示注册时提供的邮箱已被其他用户使用，
	// 用于防止同一邮箱重复注册。
	ErrEmailExists = errors.New("email already registered")

	// ErrInvalidAPIKey 表示提供的 API Key 无法匹配任何活跃用户，
	// 在 API Key 验证流程中校验失败时返回。
	ErrInvalidAPIKey = errors.New("invalid API key")
)

// Service 是认证授权模块的核心服务结构体，封装了用户认证相关的全部业务逻辑。
//
// 它持有数据库连接和 JWT 签名密钥，提供注册、登录、API Key 验证和 JWT 验证等方法。
// 在 Skynet 架构中，Service 是 Handler 和 Middleware 的底层依赖，
// 所有认证决策最终都由 Service 完成。
type Service struct {
	// db 是 GORM 数据库连接实例，用于查询和创建用户记录。
	db *gorm.DB
	// jwtSecret 是用于签发和验证 JWT 令牌的 HMAC 密钥。
	jwtSecret []byte
}

// NewService 创建并返回一个新的认证服务实例。
//
// 参数：
//   - db: GORM 数据库连接，用于用户数据的持久化存取。
//   - jwtSecret: JWT 签名密钥字符串，将被转换为字节切片用于 HMAC-SHA256 签名。
//
// 返回值：
//   - *Service: 初始化完成的认证服务实例指针。
func NewService(db *gorm.DB, jwtSecret string) *Service {
	return &Service{db: db, jwtSecret: []byte(jwtSecret)}
}

// RegisterResult 封装用户注册成功后的返回结果。
//
// 该结构体同时包含创建的用户信息和生成的明文 API Key。
// 注意：API Key 仅在注册时以明文返回一次，之后数据库中只存储其 bcrypt 哈希值，
// 用户需妥善保存此 Key，丢失后无法恢复，只能重新生成。
type RegisterResult struct {
	// User 是新创建的用户模型，包含用户 ID、邮箱、显示名等信息。
	User model.User `json:"user"`
	// APIKey 是为该用户生成的明文 API Key，格式为 "sk-" 前缀加 64 位十六进制字符串。
	APIKey string `json:"api_key"`
}

// Register 处理用户注册流程，包括邮箱唯一性检查、密码哈希、API Key 生成及用户入库。
//
// 业务流程：
//  1. 检查邮箱是否已被注册，若已存在则返回 ErrEmailExists。
//  2. 使用 bcrypt 对密码进行哈希处理。
//  3. 生成随机 API Key 并对其进行 bcrypt 哈希。
//  4. 创建用户记录并写入数据库，初始状态为 "active"。
//
// 参数：
//   - email: 用户邮箱地址，用作登录凭据，需全局唯一。
//   - password: 用户明文密码，将以 bcrypt 哈希存储。
//   - displayName: 用户显示名称，用于界面展示。
//
// 返回值：
//   - *RegisterResult: 包含新用户信息和明文 API Key 的注册结果。
//   - error: 邮箱已存在返回 ErrEmailExists，其他数据库或加密错误原样返回。
func (s *Service) Register(email, password, displayName string) (*RegisterResult, error) {
	var count int64
	s.db.Model(&model.User{}).Where("email = ?", email).Count(&count)
	if count > 0 {
		return nil, ErrEmailExists
	}

	apiKey := generateAPIKey()

	user := model.User{
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: password,
		APIKey:       apiKey,
		Status:       "active",
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}

	logger.Infof("User registered: %s (id=%d)", email, user.ID)
	return &RegisterResult{User: user, APIKey: apiKey}, nil
}

// LoginResult 封装用户登录成功后的返回结果。
//
// 包含签发的 JWT 令牌和已验证的用户信息，客户端可使用该令牌进行后续 API 请求。
type LoginResult struct {
	// Token 是签发的 JWT 令牌字符串，有效期为 24 小时，采用 HS256 签名算法。
	Token string `json:"token"`
	// User 是通过认证的用户模型，包含用户 ID、邮箱、显示名等信息。
	User model.User `json:"user"`
}

// Login 处理用户登录流程，验证邮箱和密码，成功后签发 JWT 令牌。
//
// 业务流程：
//  1. 根据邮箱查找状态为 "active" 的用户，未找到则返回 ErrUserNotFound。
//  2. 使用 bcrypt 比对密码哈希，不匹配则返回 ErrInvalidPassword。
//  3. 为通过验证的用户生成 JWT 令牌。
//
// 参数：
//   - email: 用户邮箱地址。
//   - password: 用户明文密码。
//
// 返回值：
//   - *LoginResult: 包含 JWT 令牌和用户信息的登录结果。
//   - error: 用户不存在返回 ErrUserNotFound，密码错误返回 ErrInvalidPassword，
//     JWT 生成失败返回底层错误。
func (s *Service) Login(email, password string) (*LoginResult, error) {
	var user model.User
	if err := s.db.Where("email = ? AND status = 'active'", email).First(&user).Error; err != nil {
		return nil, ErrUserNotFound
	}

	if user.PasswordHash != password {
		return nil, ErrInvalidPassword
	}

	token, err := s.generateJWT(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	return &LoginResult{Token: token, User: user}, nil
}

// ValidateAPIKey 验证客户端提供的 API Key 是否合法。
//
// 该方法遍历所有活跃用户的 API Key 哈希，逐一与提供的明文 Key 进行 bcrypt 比对。
// 这种方式确保了 API Key 不以明文存储在数据库中，提升安全性。
// Agent 通过 API Key 建立 WebSocket 隧道时，会调用此方法进行身份验证。
//
// 参数：
//   - apiKey: 客户端提供的明文 API Key 字符串。
//
// 返回值：
//   - *model.User: 匹配成功的用户模型指针。
//   - error: 无匹配用户时返回 ErrInvalidAPIKey。
func (s *Service) ValidateAPIKey(apiKey string) (*model.User, error) {
	var user model.User
	if err := s.db.Where("status = 'active' AND api_key_hash = ?", apiKey).First(&user).Error; err != nil {
		return nil, ErrInvalidAPIKey
	}
	return &user, nil
}

// ValidateJWT 验证客户端提供的 JWT 令牌是否合法，并返回对应的用户信息。
//
// 业务流程：
//  1. 解析 JWT 令牌，校验签名算法必须为 HMAC（防止算法混淆攻击）。
//  2. 验证令牌签名和有效期。
//  3. 从 Claims 中提取 user_id，查询数据库确认用户存在且状态为活跃。
//
// 参数：
//   - tokenStr: 客户端提供的 JWT 令牌字符串（不含 "Bearer " 前缀）。
//
// 返回值：
//   - *model.User: 令牌对应的用户模型指针。
//   - error: 令牌解析失败、签名无效、已过期或用户不存在时返回相应错误。
func (s *Service) ValidateJWT(tokenStr string) (*model.User, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	userID := uint64(claims["user_id"].(float64))
	var user model.User
	if err := s.db.Where("id = ? AND status = 'active'", userID).First(&user).Error; err != nil {
		return nil, ErrUserNotFound
	}

	return &user, nil
}

// generateJWT 为指定用户生成一个 JWT 令牌。
//
// 令牌采用 HS256（HMAC-SHA256）签名算法，包含以下 Claims：
//   - user_id: 用户唯一标识 ID。
//   - email: 用户邮箱地址。
//   - exp: 令牌过期时间，设定为签发后 24 小时。
//   - iat: 令牌签发时间。
//
// 参数：
//   - userID: 用户的数据库 ID。
//   - email: 用户的邮箱地址。
//
// 返回值：
//   - string: 签名后的 JWT 令牌字符串。
//   - error: 签名过程中发生的错误。
func (s *Service) generateJWT(userID uint64, email string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// RegenerateAPIKey 为指定用户重新生成 API Key。
// 旧 Key 立即失效，返回新的明文 Key（仅此一次）。
func (s *Service) RegenerateAPIKey(userID uint64) (string, error) {
	apiKey := generateAPIKey()
	if err := s.db.Model(&model.User{}).Where("id = ?", userID).
		Update("api_key_hash", apiKey).Error; err != nil {
		return "", err
	}
	return apiKey, nil
}

// generateAPIKey 生成一个加密安全的随机 API Key。
//
// 使用 crypto/rand 生成 32 字节随机数据，编码为十六进制字符串，
// 并添加 "sk-" 前缀以标识其为 Skynet 平台的 API Key。
// 最终格式为 "sk-" + 64 位十六进制字符（共 67 个字符）。
//
// 返回值：
//   - string: 生成的明文 API Key 字符串。
func generateAPIKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "sk-" + hex.EncodeToString(b)
}
