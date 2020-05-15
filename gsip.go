package gsip

import (
	"github.com/zenghr0820/gsip/sip"
)

// SIP 服务
type Service interface {
	// 返回当前配置选项
	Options() Options
	// 创建 Sip Uri 实体
	CreateSipUri(user string, domain string) sip.Uri
	// 创建请求
	CreateRequest(method sip.RequestMethod, remoteAddr string, from, to sip.Uri) sip.Request
	CreateSimpleRequest(method sip.RequestMethod, remoteAddr string) sip.Request
	// 开启监听
	Listen(network string, listenAddr string) error
	// 发送请求或者响应
	Send(message sip.Message) (sip.Transaction, error)
	// 开始 SIP 服务
	Run() error
	// 关闭服务
	Close() error
	// The service implementation
	String() string
}

func NewService(opts ...Option) Service {
	return newService(opts...)
}
