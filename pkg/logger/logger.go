package logger

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	globalLogger *zap.Logger
	sugarLogger  *zap.SugaredLogger
)

// Config 日志配置
type Config struct {
	Level      string // 日志级别: debug, info, warn, error
	FilePath   string // 日志文件路径
	MaxSize    int    // 单个日志文件最大大小(MB)
	MaxBackups int    // 保留的旧日志文件最大数量
	MaxAge     int    // 保留旧日志文件的最大天数
	Compress   bool   // 是否压缩旧日志文件
	ConsoleOut bool   // 是否同时输出到控制台
}

// DefaultConfig 返回默认日志配置。
// 默认同时写文件和控制台，方便 CLI/TUI 调试。
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	logDir := filepath.Join(homeDir, ".ai-sync-manager", "logs")

	return &Config{
		Level:      "info",
		FilePath:   filepath.Join(logDir, "app.log"),
		MaxSize:    10,
		MaxBackups: 5,
		MaxAge:     30,
		Compress:   true,
		ConsoleOut: true,
	}
}

// Init 初始化全局日志器。
// 这里会同时装配滚动文件输出和可选控制台输出。
func Init(config *Config) error {
	if config == nil {
		config = DefaultConfig()
	}

	// 确保日志目录存在
	logDir := filepath.Dir(config.FilePath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// 解析日志级别
	level := zapcore.InfoLevel
	if config.Level != "" {
		if err := level.UnmarshalText([]byte(config.Level)); err != nil {
			return err
		}
	}

	// 编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 文件输出
	fileWriter := &lumberjack.Logger{
		Filename:   config.FilePath,
		MaxSize:    config.MaxSize,
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge,
		Compress:   config.Compress,
	}

	var cores []zapcore.Core

	// 文件输出核心
	fileCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(fileWriter),
		level,
	)
	cores = append(cores, fileCore)

	// 控制台输出核心
	if config.ConsoleOut {
		consoleEncoder := encoderConfig
		consoleEncoder.EncodeLevel = zapcore.CapitalColorLevelEncoder
		consoleCore := zapcore.NewCore(
			zapcore.NewConsoleEncoder(consoleEncoder),
			zapcore.AddSync(os.Stdout),
			level,
		)
		cores = append(cores, consoleCore)
	}

	// 创建多输出核心
	core := zapcore.NewTee(cores...)

	// 创建 logger
	globalLogger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	sugarLogger = globalLogger.Sugar()

	return nil
}

// L 返回全局 logger；未初始化时会按默认配置懒加载。
func L() *zap.Logger {
	if globalLogger == nil {
		// 如果未初始化，使用默认配置初始化
		_ = Init(DefaultConfig())
	}
	return globalLogger
}

// S 返回全局 sugared logger；未初始化时会按默认配置懒加载。
func S() *zap.SugaredLogger {
	if sugarLogger == nil {
		// 如果未初始化，使用默认配置初始化
		_ = Init(DefaultConfig())
	}
	return sugarLogger
}

// Sync 刷新缓冲区，通常在进程退出前调用。
func Sync() error {
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}

// 下面这些便捷方法用于减少业务层直接触碰全局 logger 的样板代码。

// Debug 输出 debug 级别日志
func Debug(msg string, fields ...zap.Field) {
	L().Debug(msg, fields...)
}

// Info 输出 info 级别日志
func Info(msg string, fields ...zap.Field) {
	L().Info(msg, fields...)
}

// Warn 输出 warn 级别日志
func Warn(msg string, fields ...zap.Field) {
	L().Warn(msg, fields...)
}

// Error 输出 error 级别日志
func Error(msg string, fields ...zap.Field) {
	L().Error(msg, fields...)
}

// Fatal 输出 fatal 级别日志后退出
func Fatal(msg string, fields ...zap.Field) {
	L().Fatal(msg, fields...)
}

// With 创建带预设字段的 logger。
func With(fields ...zap.Field) *zap.Logger {
	return L().With(fields...)
}
