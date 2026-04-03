// Package logger 提供 Skynet 平台的全局日志工具。
//
// 该包基于 logrus 库进行封装，提供统一的日志记录接口。
// 通过包级别的函数（如 Info、Error、Debug 等）可以在项目的任何位置方便地记录日志，
// 无需手动创建和管理日志实例。日志输出到标准输出，使用带完整时间戳的文本格式。
package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

// log 是包级别的 logrus 日志实例。
// 通过 init 函数初始化，所有包级别的日志函数都通过该实例进行日志记录。
var log *logrus.Logger

// init 初始化全局日志实例。
//
// 在包被导入时自动执行，完成以下配置：
//  1. 创建新的 logrus.Logger 实例。
//  2. 设置日志输出目标为标准输出（os.Stdout）。
//  3. 设置日志格式为文本格式，并启用完整时间戳显示。
func init() {
	log = logrus.New()
	log.SetOutput(os.Stdout)
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

// SetLevel 设置全局日志级别。
//
// 支持的级别字符串包括："panic"、"fatal"、"error"、"warn"/"warning"、
// "info"、"debug"、"trace"。
// 如果传入的级别字符串无法识别，则默认使用 Info 级别。
//
// 参数：
//   - level: 日志级别字符串，不区分大小写。
func SetLevel(level string) {
	l, err := logrus.ParseLevel(level)
	if err != nil {
		l = logrus.InfoLevel
	}
	log.SetLevel(l)
}

// Info 以 Info 级别记录日志。
//
// 参数：
//   - args: 任意数量的日志内容参数，将被拼接为日志消息。
func Info(args ...interface{}) { log.Info(args...) }

// Infof 以 Info 级别记录格式化日志。
//
// 参数：
//   - format: 格式化字符串，语法与 fmt.Sprintf 一致。
//   - args: 格式化参数。
func Infof(format string, args ...interface{}) { log.Infof(format, args...) }

// Warn 以 Warn 级别记录日志，用于警告性信息。
//
// 参数：
//   - args: 任意数量的日志内容参数，将被拼接为日志消息。
func Warn(args ...interface{}) { log.Warn(args...) }

// Warnf 以 Warn 级别记录格式化日志。
//
// 参数：
//   - format: 格式化字符串，语法与 fmt.Sprintf 一致。
//   - args: 格式化参数。
func Warnf(format string, args ...interface{}) { log.Warnf(format, args...) }

// Error 以 Error 级别记录日志，用于错误信息。
//
// 参数：
//   - args: 任意数量的日志内容参数，将被拼接为日志消息。
func Error(args ...interface{}) { log.Error(args...) }

// Errorf 以 Error 级别记录格式化日志。
//
// 参数：
//   - format: 格式化字符串，语法与 fmt.Sprintf 一致。
//   - args: 格式化参数。
func Errorf(format string, args ...interface{}) { log.Errorf(format, args...) }

// Debug 以 Debug 级别记录日志，用于调试信息。
//
// 参数：
//   - args: 任意数量的日志内容参数，将被拼接为日志消息。
func Debug(args ...interface{}) { log.Debug(args...) }

// Debugf 以 Debug 级别记录格式化日志。
//
// 参数：
//   - format: 格式化字符串，语法与 fmt.Sprintf 一致。
//   - args: 格式化参数。
func Debugf(format string, args ...interface{}) { log.Debugf(format, args...) }

// Fatal 以 Fatal 级别记录日志，并在记录后调用 os.Exit(1) 终止程序。
//
// 参数：
//   - args: 任意数量的日志内容参数，将被拼接为日志消息。
func Fatal(args ...interface{}) { log.Fatal(args...) }

// Fatalf 以 Fatal 级别记录格式化日志，并在记录后调用 os.Exit(1) 终止程序。
//
// 参数：
//   - format: 格式化字符串，语法与 fmt.Sprintf 一致。
//   - args: 格式化参数。
func Fatalf(format string, args ...interface{}) { log.Fatalf(format, args...) }

// WithField 创建一个附带单个键值对字段的日志条目。
//
// 返回的 logrus.Entry 可用于链式调用，继续添加字段或记录日志。
// 适用于需要在日志中携带结构化上下文信息的场景。
//
// 参数：
//   - key: 字段名称。
//   - value: 字段值。
//
// 返回值：
//   - *logrus.Entry: 附带指定字段的日志条目，可继续链式调用。
func WithField(key string, value interface{}) *logrus.Entry { return log.WithField(key, value) }

// WithFields 创建一个附带多个键值对字段的日志条目。
//
// 返回的 logrus.Entry 可用于链式调用，继续添加字段或记录日志。
// 适用于需要在日志中同时携带多个结构化上下文信息的场景。
//
// 参数：
//   - fields: logrus.Fields 类型的字段映射（map[string]interface{}）。
//
// 返回值：
//   - *logrus.Entry: 附带指定字段的日志条目，可继续链式调用。
func WithFields(fields logrus.Fields) *logrus.Entry { return log.WithFields(fields) }
