package transport

import (
	"fmt"
	"net"
	"strings"

	"github.com/zenghr0820/gsip/logger"
	"github.com/zenghr0820/gsip/sip"
)

/**
创建 TCP 协议：
	receiveMessage: 传输层接收 Tcp 协议数据的 chan
	receiveError: 传输层接收 Tcp 协议异常的 chan
	notifyCancel：传输层通知 Tcp 协议关闭的 chan
*/
func CreateTcpProtocol(
	receiveMessage chan<- sip.Message,
	receiveError chan<- error,
	notifyCancel <-chan struct{},
) Protocol {
	tcp := new(tcpProtocol)
	tcp.network = "tcp"
	tcp.reliable = true
	tcp.streamed = true
	tcp.receiveConnection = make(chan Connection)

	// TODO: add separate errs chan to listen errors from pool for reconnection?
	tcp.listeners = CreateListenerPool(tcp.receiveConnection, receiveError, notifyCancel)
	tcp.connections = CreateConnectionPool(receiveMessage, receiveError, notifyCancel)
	// pipe listener and connection pools
	// 添加新的连接到连接池
	go tcp.pipePools()
	return tcp
}

// TCP protocol implementation
type tcpProtocol struct {
	protocol
	connections       ConnectionPool
	listeners         ListenerPool
	receiveConnection chan Connection
}

func (tcp *tcpProtocol) Done() <-chan struct{} {
	return tcp.connections.Done()
}

// 解析地址
func (tcp *tcpProtocol) resolveAddr(addr *sip.Addr) (*net.TCPAddr, error) {
	network := strings.ToLower(tcp.Network())
	address := addr.Addr()

	remoteAddr, err := net.ResolveTCPAddr(network, address)
	if err != nil {
		return nil, &ProtocolError{
			err,
			fmt.Sprintf("[tcp_protocol] -> resolve target address %s %s", tcp.Network(), remoteAddr),
			fmt.Sprintf("%p", tcp),
		}
	}
	return remoteAddr, err
}

// 监听
func (tcp *tcpProtocol) Listen(addr *sip.Addr) error {
	addr = sip.FillTargetHostAndPort(tcp.Network(), addr)
	network := strings.ToLower(tcp.Network())
	// resolve local TCP endpoint
	localAddr, err := tcp.resolveAddr(addr)
	if err != nil {
		return err
	}
	listener, err := net.ListenTCP(network, localAddr)
	if err != nil {
		return &ProtocolError{
			err,
			fmt.Sprintf("[tcp_protocol] -> listen on %s %s address", tcp.Network(), localAddr),
			fmt.Sprintf("%p", tcp),
		}
	}

	logger.Infof("[tcp_protocol] -> begin listen on %s %s", tcp.Network(), localAddr)

	// 创建连接
	key := ListenerKey(fmt.Sprintf("tcp:0.0.0.0:%d", localAddr.Port))
	// 将监听的连接添加进 连接池
	err = tcp.listeners.Put(key, listener)
	return err
}

// 发送
func (tcp *tcpProtocol) Send(rAddr *sip.Addr, msg sip.Message) error {
	// 验证发送地址
	if rAddr.Host == "" {
		return &ProtocolError{
			fmt.Errorf("[tcp_protocol] -> empty remote target host"),
			fmt.Sprintf("[tcp_protocol] -> send SIP message to %s %s", tcp.Network(), rAddr.Addr()),
			fmt.Sprintf("%p", tcp),
		}
	}
	// 解析地址
	remoteAddr, err := tcp.resolveAddr(rAddr)
	if err != nil {
		return err
	}

	// 获取连接
	conn, err := tcp.getOrCreateConnection(remoteAddr)
	if err != nil {
		return err
	}

	logger.Debugf("[tcp_protocol] -> writing SIP message to %s %s", tcp.Network(), remoteAddr)

	// send message
	_, err = conn.Write([]byte(msg.String()))

	return err // should be nil
}

// 添加新的连接到连接池
func (tcp *tcpProtocol) pipePools() {
	defer close(tcp.receiveConnection)

	logger.Debug("[tcp_protocol] -> start pipe pools")
	defer logger.Debug("[tcp_protocol] -> stop pipe pools")

	for {
		select {
		case <-tcp.listeners.Done():
			return
		case conn := <-tcp.receiveConnection:
			if err := tcp.connections.Put(conn, SockTTL); err != nil {
				// TODO should it be passed up to UA?
				logger.Errorf("[tcp_protocol] -> put new TCP connection failed: %s", err)

				continue
			}
		}
	}
}

// 获取连接或者创建连接
func (tcp *tcpProtocol) getOrCreateConnection(remoteAddr *net.TCPAddr) (Connection, error) {
	network := strings.ToLower(tcp.Network())

	key := ConnectionKey("tcp:" + remoteAddr.String())
	conn, err := tcp.connections.Get(key)
	if err != nil {
		logger.Debugf("[tcp_protocol] -> connection for remote address %s %s not found, create a new one", tcp.Network(), remoteAddr)

		tcpConn, err := net.DialTCP(network, nil, remoteAddr)
		if err != nil {
			return nil, &ProtocolError{
				err,
				fmt.Sprintf("[tcp_protocol] -> connect to %s %s address", tcp.Network(), remoteAddr),
				fmt.Sprintf("%p", tcp),
			}
		}

		conn = CreateConnection(key, tcpConn)
		if err := tcp.connections.Put(conn, SockTTL); err != nil {
			return conn, err
		}
	}

	return conn, nil
}
