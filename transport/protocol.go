package transport

import (
	"strings"

	"github.com/zenghr0820/gsip/sip"
)

type protocolKey string

func (key protocolKey) String() string {
	return strings.ToUpper(string(key))
}

// 协议层 Protocol
type Protocol interface {
	Network() string
	Listen(addr *sip.Addr) error                // 监听
	Send(addr *sip.Addr, msg sip.Message) error // 发送
	Done() <-chan struct{}
	Reliable() bool
	Streamed() bool
}

type protocol struct {
	network  string
	reliable bool
	streamed bool
}

func (p *protocol) Network() string {
	return strings.ToUpper(p.network)
}

func (p *protocol) Reliable() bool {
	return p.reliable
}

func (p *protocol) Streamed() bool {
	return p.streamed
}
