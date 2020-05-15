package logger

import zapLogger "github.com/zenghr0820/zap-logger"

var (
	logger zapLogger.Logger
)

type Logger interface {
	// 初始化 callback
	Init(opts ...Option)
	// logger 的配置选项
	Options() *Options

	String() string
}
