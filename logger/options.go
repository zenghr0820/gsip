package logger

import (
	zapLogger "github.com/zenghr0820/zap-logger"
)

// logger 配置
type Options struct {
	Name    string
	Dir     string
	Level   zapLogger.LogLevel
	EnvMode string
	Skip    int
}

type Option func(o *Options)

func Name(name string) Option {
	return func(o *Options) {
		o.Name = name
	}
}

func Dir(dir string) Option {
	return func(o *Options) {
		o.Dir = dir
	}
}

func Level(level string) Option {
	return func(o *Options) {
		o.Level = zapLogger.LogLevel(level)
	}
}

func EnvMode(env string) Option {
	return func(o *Options) {
		o.EnvMode = env
	}
}

func Skip(skip int) Option {
	return func(o *Options) {
		o.Skip = skip
	}
}
