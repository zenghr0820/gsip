package logger

import (
	"sync"

	zapLogger "github.com/zenghr0820/zap-logger"
)

type defaultLogger struct {
	opts Options
	once sync.Once
}

func NewLogger(opts ...Option) Logger {
	options := Options{
		Name:    "gsip",
		Dir:     "",
		Level:   zapLogger.ErrorLevel,
		EnvMode: "prod",
		Skip:    2,
	}

	for _, o := range opts {
		o(&options)
	}

	return &defaultLogger{
		opts: options,
	}
}

func (d *defaultLogger) Init(opts ...Option) {
	for _, o := range opts {
		o(&d.opts)
	}

	d.once.Do(func() {
		logger = zapLogger.InitLog(&zapLogger.Config{
			Name:    d.opts.Name,
			Dir:     d.opts.Dir,
			Level:   d.opts.Level,
			EnvMode: d.opts.EnvMode,
			Skip:    d.opts.Skip,
		})
	})
}

func (d *defaultLogger) Options() *Options {
	return &d.opts
}

func (d *defaultLogger) String() string {
	return d.opts.Name + "-" + d.opts.EnvMode
}
