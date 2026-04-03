// Package database 提供 Skynet 平台的数据库连接管理功能。
//
// 当前支持 MySQL 数据库，使用 GORM 作为 ORM 框架。
// 该包封装了数据库连接的初始化和连接池配置，
// 为 Skynet 平台的各个服务模块提供统一的数据库访问入口。
package database

import (
	"github.com/skynetplatform/skynet/internal/config"
	"github.com/skynetplatform/skynet/pkg/logger"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// NewMySQL 根据给定的数据库配置创建并返回一个 GORM 数据库连接实例。
//
// 该函数执行以下操作：
//  1. 使用配置中的 DSN（Data Source Name）建立 MySQL 连接。
//  2. 配置连接池参数：最大打开连接数为 50，最大空闲连接数为 10。
//  3. 连接成功后通过日志记录连接状态。
//
// 参数：
//   - cfg: 数据库配置对象，包含 DSN 等连接信息，通过其 DSN() 方法获取连接字符串。
//
// 返回值：
//   - *gorm.DB: 初始化完成的 GORM 数据库连接实例，可直接用于数据库操作。
//   - error: 如果连接建立失败或连接池配置失败则返回错误，否则为 nil。
func NewMySQL(cfg config.DatabaseConfig) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(10)

	logger.Info("MySQL connected")
	return db, nil
}
