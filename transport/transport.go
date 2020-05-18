package transport

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/zenghr0820/gsip/logger"
	"github.com/zenghr0820/gsip/sip"
)

var (
	DefaultAddress = net.ParseIP("127.0.0.1")
)

// 创建传输层
// - opts - 传输层配置
func CreateLayer(opts ...Option) Layer {
	tpl := &layer{
		protocols:      createProtocolPool(),
		upMessage:      make(chan sip.Message),
		upError:        make(chan error),
		receiveMessage: make(chan sip.Message),
		receiveError:   make(chan error),
		cancel:         make(chan struct{}),
		done:           make(chan struct{}),
	}

	// 配置
	tpl.opts = newOptions(opts...)

	// 监听协议层通知
	go tpl.listenProtocolNotice()
	return tpl
}

// 定义传输层接口 TransportLayer
type Layer interface {
	// 初始化 callback
	Init(opts ...Option)
	// 判断是否是可靠的
	IsReliable(network string) bool
	// 监听
	Listen(network string, addr string) error
	// 返回消息通知
	GetMessage() <-chan sip.Message
	// 返回异常
	Errors() <-chan error
	// 发送
	Send(message sip.Message) error
	// 关闭
	Close()
	// 确认关闭是否完成
	Done() <-chan struct{}
}

// 实例化传输层
type layer struct {
	opts Options
	// 传输层实例化的协议层
	protocols *protocolPool
	// 向上传递消息
	upMessage chan sip.Message
	// 向上传递异常
	upError chan error
	// 监听接收消息
	receiveMessage chan sip.Message
	// 监听接收异常
	receiveError chan error

	cancel chan struct{}
	done   chan struct{}

	wg sync.WaitGroup
}

func (tpl *layer) Init(opts ...Option) {
	for _, o := range opts {
		o(&tpl.opts)
	}

	if tpl.opts.localIP == nil {
		LocalAddr("")(&tpl.opts)
	}

	if tpl.opts.dnsResolver == nil {
		DnsResolverConfig("")(&tpl.opts)
	}
}

func (tpl *layer) Done() <-chan struct{} {
	return tpl.done
}

func (tpl *layer) IsReliable(network string) bool {
	if protocol, ok := tpl.protocols.get(protocolKey(network)); ok && protocol.Reliable() {
		return true
	}
	return false
}

func (tpl *layer) GetMessage() <-chan sip.Message {
	return tpl.upMessage
}

func (tpl *layer) Errors() <-chan error {
	return tpl.upError
}

// 监听
func (tpl *layer) Listen(network string, addr string) error {
	select {
	case <-tpl.cancel:
		return fmt.Errorf("[tpl_layer] -> transport layer is canceled")
	default:
	}

	// 检查 协议池是否有该协议，有则取出，无则创建添加进协议池
	protocol, ok := tpl.protocols.get(protocolKey(network))
	if !ok {
		var err error
		protocol, err = protocolFactory(network, tpl.receiveMessage, tpl.receiveError, tpl.cancel)
		if err != nil {
			return err
		}
		tpl.protocols.put(protocolKey(network), protocol)
	}

	// 格式化地址
	lAddr, err := sip.NewTargetFromAddr(addr)
	if err != nil {
		return err
	}

	// 填充默认值
	lAddr = sip.FillTargetHostAndPort(network, lAddr)
	return protocol.Listen(lAddr)
}

// 发送
func (tpl *layer) Send(message sip.Message) error {
	select {
	case <-tpl.cancel:
		return fmt.Errorf("[tpl_layer] -> transport layer is canceled")
	default:
	}

	viaHop, ok := message.ViaHop()
	if !ok {
		return &sip.MalformedMessageError{
			Err: fmt.Errorf("[tpl_layer] -> missing required 'Via' header"),
			Msg: message.String(),
		}
	}

	switch msg := message.(type) {
	// RFC 3261 - 18.1.1.
	case sip.Request:
		nets := make([]string, 0)
		msgLen := len(msg.String())
		// todo check for reliable/non-reliable
		// 检查是可靠还是不可靠传输
		if msgLen > int(MTU)-200 {
			nets = append(nets, "tcp", "udp")
		} else {
			nets = append(nets, "udp", "tcp")
		}

		viaHop.Host = tpl.opts.localIP.String()
		if viaHop.Params == nil {
			viaHop.Params = sip.NewParams()
		}
		if !viaHop.Params.Has("rport") {
			viaHop.Params.Add("rport", nil)
		}
		var err error
		for _, nt := range nets {
			protocol, ok := tpl.protocols.get(protocolKey(nt))
			if !ok {
				err = UnsupportedProtocolError(fmt.Sprintf("[tpl_layer] -> protocol %s is not supported", nt))
				continue
			}
			// rewrite sent-by transport
			viaHop.Transport = strings.ToUpper(nt)
			// rewrite sent-by port
			// TODO if port, ok := tpl.listenPorts[nt]; ok
			defPort := sip.DefaultPort(nt)
			viaHop.Port = &defPort

			var target *sip.Addr
			//logger.Info("msg.Destination() -> ", msg.Destination())
			target, err = sip.NewTargetFromAddr(msg.Destination())
			if err != nil {
				continue
			}

			// dns srv lookup
			if net.ParseIP(target.Host) == nil {
				ctx := context.Background()
				proto := strings.ToLower(nt)
				if _, adders, err := tpl.opts.dnsResolver.LookupSRV(ctx, "sip", proto, target.Host); err == nil && len(adders) > 0 {
					addr := adders[0]
					addrStr := fmt.Sprintf("%s:%d", addr.Target[:len(addr.Target)-1], addr.Port)
					switch nt {
					case "UDP":
						if addr, err := net.ResolveUDPAddr("udp", addrStr); err == nil {
							port := sip.Port(addr.Port)
							target.Host = addr.IP.String()
							target.Port = &port
						}
					case "TCP":
						if addr, err := net.ResolveTCPAddr("tcp", addrStr); err == nil {
							port := sip.Port(addr.Port)
							target.Host = addr.IP.String()
							target.Port = &port
						}
					}
				}
			}

			logger.Debugf("[tpl_layer] -> sending SIP request:\n%s", msg)

			err = protocol.Send(target, msg)
			if err == nil {
				break
			} else {
				continue
			}
		}

		return err
		// RFC 3261 - 18.2.2.
	case sip.Response:
		// resolve protocol from Via
		protocol, ok := tpl.protocols.get(protocolKey(viaHop.Transport))
		if !ok {
			return UnsupportedProtocolError(fmt.Sprintf("[tpl_layer] -> protocol %s is not supported", viaHop.Transport))
		}

		target, err := sip.NewTargetFromAddr(msg.Destination())
		if err != nil {
			return err
		}

		logger.Debugf("[tpl_layer] -> send SIP response:\n%s", msg)

		return protocol.Send(target, msg)
	default:
		return &sip.UnsupportedMessageError{
			Err: fmt.Errorf("[tpl_layer] -> unsupported message %s", msg.Short()),
			Msg: msg.String(),
		}
	}
}

func (tpl *layer) Close() {
	select {
	case <-tpl.cancel:
		return
	default:
	}

	close(tpl.cancel)
}

// 释放资源
func (tpl *layer) release() {
	logger.Info("[tpl_layer] -> release resources")
	// wait for protocols
	for _, protocol := range tpl.protocols.all() {
		tpl.protocols.del(protocolKey(protocol.Network()))
		<-protocol.Done()
	}

	close(tpl.receiveMessage)
	close(tpl.receiveError)
	close(tpl.upMessage)
	close(tpl.upError)
}

// 处理监听的信息
func (tpl *layer) handleMessage(message sip.Message) {
	logger.Debug("[tpl_layer] -> received SIP message [Protocol]")
	logger.Debug("[tpl_layer] -> passing up SIP message...")

	// 往上层抛消息
	select {
	case <-tpl.cancel:
	case tpl.upMessage <- message:
		logger.Debug("[tpl_layer] -> SIP message passed up [transaction]")
	}
}

// 处理监听的异常
func (tpl *layer) handleError(err error) {
	logger.Info("[tpl_layer] -> received SIP error")
	logger.Debug("[tpl_layer] -> passing up error...")

	// 往上层抛消息
	select {
	case <-tpl.cancel:
	case tpl.upError <- err:
		logger.Info("[tpl_layer] -> error passed up [transaction]")
	}
}

// 监听协议层通知
func (tpl *layer) listenProtocolNotice() {
	defer func() {
		// 释放资源
		tpl.release()
		close(tpl.done)
	}()

	logger.Info("[tpl_layer] -> begin listen protocol message")
	defer logger.Info("[tpl_layer] -> stop listen protocol message")

	for {
		select {
		case <-tpl.cancel:
			return
		case msg := <-tpl.receiveMessage:
			tpl.handleMessage(msg)
		case err := <-tpl.receiveError:
			tpl.handleError(err)
		}
	}
}
