package gsip

import (
	"github.com/zenghr0820/gsip/callback"
	"github.com/zenghr0820/gsip/logger"
	"github.com/zenghr0820/gsip/sip"
	"github.com/zenghr0820/gsip/transaction"
	"github.com/zenghr0820/gsip/transport"
)

// Options for sip service
// SIP 服务选项 callback
type Options struct {
	// 事务层
	tx transaction.Layer
	// 传输层
	tp transport.Layer
	// 回调处理函数
	Callback callback.Callback
	// 日志
	Logger logger.Logger
}

type Option func(*Options)
type LoggerOption func(*logger.Options)

func newOptions(opts ...Option) Options {
	opt := Options{
		Callback: callback.DefaultCallback,
		tp:       transport.CreateLayer(),
		Logger:   logger.NewLogger(),
	}
	opt.tx = transaction.CreateLayer(opt.tp)

	for _, o := range opts {
		o(&opt)
	}

	return opt
}

// 配置请求回调函数
func RequestCallback(callback map[sip.RequestMethod]sip.RequestHandler) Option {
	return func(o *Options) {
		err := o.Callback.SetRequestHandle(callback)
		logger.Error(err)
	}
}

// 配置响应回调函数
func ResponseCallback(callback sip.ResponseHandler) Option {
	return func(o *Options) {
		err := o.Callback.SetResponseHandle(callback)
		logger.Error(err)
	}
}

// 配置请求回调函数
func AddRequestCallback(method sip.RequestMethod, handler sip.RequestHandler) Option {
	return func(o *Options) {
		o.Callback.AddRequestHandle(method, handler)
	}
}

// 配置响应回调函数
// func AddResponseCallback(method sip.ResponseMethod, handler sip.ResponseHandler) Option {
// 	return func(o *Options) {
// 		o.Callback.AddResponseHandle(method, handler)
// 	}
// }

// 配置传输层IP地址
func Transport(localhost string) Option {
	return func(o *Options) {
		o.tp.Init(transport.LocalAddr(localhost))
	}
}

// 配置传输层 DNS
func DnsConfig(dns string) Option {
	return func(o *Options) {
		o.tp.Init(transport.DnsResolverConfig(dns))
	}
}

// 配置日志
func LoggerConfig(opts ...LoggerOption) Option {
	return func(o *Options) {
		for _, opt := range opts {
			opt(o.Logger.Options())
		}
		o.Logger.Init()
	}
}

func LoggerName(name string) LoggerOption {
	return func(o *logger.Options) {
		option := logger.Name(name)
		option(o)
	}
}

func LoggerDir(dir string) LoggerOption {
	return func(o *logger.Options) {
		option := logger.Dir(dir)
		option(o)
	}
}

func LoggerLevel(level string) LoggerOption {
	return func(o *logger.Options) {
		option := logger.Level(level)
		option(o)
	}
}

func LoggerEnv(env string) LoggerOption {
	return func(o *logger.Options) {
		option := logger.EnvMode(env)
		option(o)
	}
}

func LoggerSkip(skip int) LoggerOption {
	return func(o *logger.Options) {
		option := logger.Skip(skip)
		option(o)
	}
}
