package transport

import (
	"strings"

	"github.com/zenghr0820/gsip/sip"
)

// 实例化协议工厂
// receiveMessage: 传输层用于接收 协议层数据的 chan
// receiveError: 传输层用于接收 协议层异常的 chan
// notifyCancel：传输层用于通知 协议层关闭的 chan
var protocolFactory = func(
	network string,
	receiveMessage chan<- sip.Message,
	receiveError chan<- error,
	notifyCancel <-chan struct{},
) (Protocol, error) {
	switch strings.ToLower(network) {
	case "udp":
		return CreateUdpProtocol(receiveMessage, receiveError, notifyCancel), nil
	case "tcp":
		return CreateTcpProtocol(receiveMessage, receiveError, notifyCancel), nil
	default:
		return nil, nil
	}
}
