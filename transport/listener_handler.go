package transport

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/zenghr0820/gsip/logger"
)

// 关联监听，管理对应的监听连接
type ListenerKey string

func (key ListenerKey) String() string {
	return string(key)
}

/**
实例化监听管理服务：
	key: 监听对应的唯一 Key
	listener: 监听实例
	receiveConnection：接收监听到的连接
	receiveError：接收异常的 chan
*/
func CreateListenerHandler(
	key ListenerKey,
	listener net.Listener,
	receiveConnection chan<- Connection,
	receiveError chan<- error,
) ListenerHandler {
	handler := &listenerHandler{
		key:         key,
		listener:    listener,
		output:      receiveConnection,
		handleError: receiveError,
		cancel:      make(chan struct{}),
	}

	return handler
}

// 定义管理接口
type ListenerHandler interface {
	// 返回该监听的唯一 Key
	Key() ListenerKey
	// 返回关联的监听
	Listener() net.Listener
	// 监听管理：获取连接
	ListenerServer(done func())
	// 释放资源、关闭等操作
	Close()
}

// 监听管理实现
type listenerHandler struct {
	// 监听唯一 Key
	key ListenerKey
	// 关联的监听
	listener net.Listener
	// 传递监听的到连接
	output chan<- Connection
	// 传递异常
	handleError chan<- error
	// 确保只执行一次关闭操作
	cancelOnce sync.Once
	// 关闭通知
	cancel chan struct{}
}

func (handler *listenerHandler) Key() ListenerKey {
	return handler.key
}

func (handler *listenerHandler) Listener() net.Listener {
	return handler.listener
}

// 执行监听 Accept 获取连接上来的连接
func (handler *listenerHandler) ListenerServer(done func()) {
	defer func() {
		// 关闭
		handler.Close()
		// 通知监听池已完成
		done()
	}()
	logger.Infof("[listener_handler] -> begin listener(%s) Handler Server", handler.key)
	defer logger.Infof("[listener_handler] -> stop listener(%s) Handler Server", handler.key)

	// 开始监听服务
	connectionChan := make(chan Connection)
	readError := make(chan error)

	// 等待 goroutine 结束
	wg := sync.WaitGroup{}
	wg.Add(2)

	// 启动 goroutine 获取连接
	go handler.acceptConnections(&wg, connectionChan, readError)
	// 启动 goroutine 传输连接
	go handler.pipeOutputs(&wg, connectionChan, readError)

	wg.Wait()
}

// 获取连接
func (handler *listenerHandler) acceptConnections(wg *sync.WaitGroup, connectionChan chan<- Connection, readError chan<- error) {
	defer func() {
		close(connectionChan)
		close(readError)

		wg.Done()
	}()
	logger.Debugf("[listener_handler] -> (%s)begin accept connections", handler.key)
	defer logger.Debugf("[listener_handler] -> (%s)stop accept connections", handler.key)
	// 开始读取
	for {
		// wait for the new connection
		baseConn, err := handler.Listener().Accept()
		if err != nil {
			// if we get timeout error just go further and try accept on the next iteration
			var netErr net.Error
			if errors.As(err, &netErr) {
				if netErr.Timeout() || netErr.Temporary() {
					logger.Warnf("listener timeout or temporary unavailable, sleep by %s", NetErrRetryTime)

					time.Sleep(NetErrRetryTime)

					continue
				}
			}

			// broken or closed listener
			// pass up error and exit
			select {
			case <-handler.cancel:
			case readError <- err:
			}
			return
		}

		key := ConnectionKey(strings.ToLower(baseConn.RemoteAddr().Network()) + ":" + baseConn.RemoteAddr().String())
		conn := CreateConnection(key, baseConn)

		select {
		case <-handler.cancel:
			return
		case connectionChan <- conn:
		}
	}

}

// 输出连接
func (handler *listenerHandler) pipeOutputs(wg *sync.WaitGroup, connectionChan <-chan Connection, readError <-chan error) {
	defer wg.Done()

	logger.Debug("[listener_handle] -> begin pipe outputs")
	defer logger.Debug("[listener_handle] -> stop pipe outputs")

	for {
		select {
		case <-handler.cancel:
			return
		case conn, ok := <-connectionChan:
			// chan closed
			if !ok {
				return
			}
			if conn != nil {

				logger.Debug("[listener_handle] -> passing up connection...")

				select {
				case <-handler.cancel:
					return
				case handler.output <- conn:
					logger.Info("[listener_handle] -> connection passed up")
				}
			}
		case err, ok := <-readError:
			// chan closed
			if !ok {
				return
			}
			if err != nil {
				var listenerHandlerError *ListenerHandlerError
				if !errors.As(err, &listenerHandlerError) {
					err = &ListenerHandlerError{
						err,
						handler.Key(),
						fmt.Sprintf("%p", handler),
						listenerNetwork(handler.Listener()),
						handler.Listener().Addr().String(),
					}
				}

				logger.Debug("[listener_handle] -> passing up listener error...")

				select {
				case <-handler.cancel:
					return
				case handler.handleError <- err:
					logger.Info("[listener_handle] -> error passed up")
				}
			}
		}
	}
}

// 执行释放资源、关闭等操作
func (handler *listenerHandler) Close() {
	select {
	case <-handler.cancel:
		return
	default:
	}

	handler.cancelOnce.Do(func() {
		close(handler.cancel)

		if err := handler.Listener().Close(); err != nil {
			logger.Errorf("[listener_handler] -> listener close failed: %s", err)
		}
	})
}

// 返回监听的协议
func listenerNetwork(ls net.Listener) string {
	switch ls.(type) {
	case *net.TCPListener:
		return "tcp"
	case *net.UnixListener:
		return "unix"
	default:
		return ""
	}
}
