package transport

import (
	"fmt"
	"net"
	"strings"

	"github.com/zenghr0820/gsip/logger"
	"github.com/zenghr0820/gsip/sip"
)

/**
创建 UDP 协议：
	receiveMessage: 传输层接收 Udp 协议数据的 chan
	receiveError: 传输层接收 Udp 协议异常的 chan
	notifyCancel：传输层通知 Udp 协议关闭的 chan
*/
func CreateUdpProtocol(
	receiveMessage chan<- sip.Message,
	receiveError chan<- error,
	notifyCancel <-chan struct{},
) Protocol {
	udp := new(udpProtocol)
	udp.network = "udp"
	udp.reliable = false
	udp.streamed = false

	// TODO: add separate errs chan to listen errors from pool for reconnection?
	udp.connections = CreateConnectionPool(receiveMessage, receiveError, notifyCancel)
	return udp
}

// UDP protocol implementation
type udpProtocol struct {
	protocol
	connections ConnectionPool
}

func (udp *udpProtocol) Done() <-chan struct{} {
	return udp.connections.Done()
}

// 解析地址
func (udp *udpProtocol) resolveAddr(addr *sip.Addr) (*net.UDPAddr, error) {
	network := strings.ToLower(udp.Network())
	address := addr.Addr()

	remoteAddr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return nil, &ProtocolError{
			err,
			fmt.Sprintf("[udp_protocol] -> resolve target address %s %s", udp.Network(), remoteAddr),
			fmt.Sprintf("%p", udp),
		}
	}
	return remoteAddr, err
}

// 监听
func (udp *udpProtocol) Listen(addr *sip.Addr) error {
	addr = sip.FillTargetHostAndPort(udp.Network(), addr)
	network := strings.ToLower(udp.Network())
	// resolve local UDP endpoint
	localAddr, err := udp.resolveAddr(addr)
	if err != nil {
		return err
	}
	udpConn, err := net.ListenUDP(network, localAddr)
	if err != nil {
		return &ProtocolError{
			err,
			fmt.Sprintf("[udp_protocol] -> listen on %s %s address", udp.Network(), localAddr),
			fmt.Sprintf("%p", udp),
		}
	}

	logger.Infof("[udp_protocol] -> begin listening on %s %s", udp.Network(), localAddr)

	// 创建连接
	key := ConnectionKey(fmt.Sprintf("udp:0.0.0.0:%d", localAddr.Port))
	conn := CreateConnection(key, udpConn)
	// 将监听的连接添加进 连接池
	err = udp.connections.Put(conn, 0)
	return err
}

// 发送
func (udp *udpProtocol) Send(rAddr *sip.Addr, msg sip.Message) error {
	// 验证发送地址
	if rAddr.Host == "" {
		return &ProtocolError{
			fmt.Errorf("[udp_protocol] -> empty remote target host"),
			fmt.Sprintf("[udp_protocol] -> send SIP message to %s %s", udp.Network(), rAddr.Addr()),
			fmt.Sprintf("%p", udp),
		}
	}
	// 解析地址
	remoteAddr, err := udp.resolveAddr(rAddr)
	if err != nil {
		return err
	}

	// send through already opened by connection
	// to always use same local port
	// 通过已打开的连接发送，始终使用相同的本地端口
	_, port, err := net.SplitHostPort(msg.Source())
	conn, err := udp.connections.Get(ConnectionKey("udp:0.0.0.0:" + port))
	if err != nil {
		// todo change this bloody patch
		if len(udp.connections.All()) == 0 {
			return &ProtocolError{
				fmt.Errorf("[udp_protocol] -> connection not found: %w", err),
				fmt.Sprintf("[udp_protocol] -> send SIP message to %s %s", udp.Network(), remoteAddr),
				fmt.Sprintf("%p", udp),
			}
		}

		conn = udp.connections.All()[0]
	}

	logger.Debugf("[udp_protocol] -> writing SIP message to %s %s", udp.Network(), remoteAddr)

	_, err = conn.WriteTo([]byte(msg.String()), remoteAddr)

	return err // should be nil
}
