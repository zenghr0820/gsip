package transport

import (
	"context"
	"net"

	"github.com/zenghr0820/gsip/logger"
	"github.com/zenghr0820/gsip/utils"
)

// transport 的配置选项
type Options struct {
	// 连接本地的IP地址
	localIP net.IP
	// DNS 配置
	dnsResolver *net.Resolver
}

type Option func(o *Options)

func newOptions(opts ...Option) Options {
	opt := Options{}

	for _, o := range opts {
		o(&opt)
	}

	if opt.localIP == nil {
		if v, err := utils.GetLocalRealIp(); err == nil {
			opt.localIP = v
		} else {
			logger.Warnf("[transport_option] -> resolve host IP failed: %s", err)
			opt.localIP = DefaultAddress
		}
	}

	if opt.localIP == nil {
		opt.dnsResolver = net.DefaultResolver
	}

	return opt
}

// 配置传输层IP地址
func LocalAddr(localhost string) Option {
	return func(o *Options) {
		var ip net.IP
		if localhost != "" {
			if addr, err := net.ResolveIPAddr("ip", localhost); err == nil {
				ip = addr.IP
			} else {
				logger.Panicf("[transport_option] -> resolve host IP failed: %s", err)
			}
			o.localIP = ip
		}
	}
}

// 配置 DNS
func DnsResolverConfig(dns string) Option {
	return func(o *Options) {
		var dnsResolver *net.Resolver
		if dns != "" {
			dnsResolver = &net.Resolver{
				PreferGo: true,
				Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
					d := net.Dialer{}
					return d.DialContext(ctx, "udp", dns)
				},
			}
			o.dnsResolver = dnsResolver
		}
	}
}
