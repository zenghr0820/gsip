package transport

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/zenghr0820/gsip/logger"
	"github.com/zenghr0820/gsip/sip"
)

// 关联连接，管理对应的连接

/**
实例化连接管理：
	conn: 连接实例
	ttl: 连接过期时间
	receiveMessage：接收数据的 chan
	receiveError：接收异常的 chan
*/
func CreateConnectionHandler(
	conn Connection,
	ttl time.Duration,
	receiveMessage chan<- sip.Message,
	receiveError chan<- error,
) ConnectionHandler {
	handler := &connectionHandler{
		key:         conn.Key(),
		connection:  conn,
		output:      receiveMessage,
		handleError: receiveError,
		cancel:      make(chan struct{}),
		ttl:         ttl,
	}

	// handler.Update(ttl)
	if ttl > 0 {
		handler.expiry = time.Now().Add(ttl)
		handler.exTimer = time.NewTimer(ttl)
	} else {
		handler.expiry = time.Time{}
		handler.exTimer = time.NewTimer(0)
		handler.exTimer.Stop()
	}

	return handler
}

// 定义管理接口
type ConnectionHandler interface {
	// 返回该连接的唯一 Key
	Key() ConnectionKey
	// 返回关联的连接
	Connection() Connection
	// 返回连接过期时间
	Expiry() time.Time
	// 是否过期
	IsExpiry() bool
	// 连接管理：管理过期时间，读取数据，输入输出等
	ConnectionServer(done func())
	// 释放资源、关闭等操作
	Close()
}

// 连接管理实现
type connectionHandler struct {
	// 连接唯一 Key
	key ConnectionKey
	// 关联的连接
	connection Connection
	// 传递输出数据
	output chan<- sip.Message
	// 传递异常
	handleError chan<- error
	// 过期时间
	expiry time.Time
	// 过期定时器
	exTimer *time.Timer
	// 过期时长
	ttl time.Duration
	// 确保只执行一次关闭操作
	cancelOnce sync.Once
	// 关闭通知
	cancel chan struct{}
}

func (handler *connectionHandler) Key() ConnectionKey {
	return handler.key
}

func (handler *connectionHandler) Connection() Connection {
	return handler.connection
}

func (handler *connectionHandler) Expiry() time.Time {
	return handler.expiry
}

func (handler *connectionHandler) IsExpiry() bool {
	// 过期：过期时间时间不为空 并且 过期时间在当前时间之前
	return !handler.Expiry().IsZero() && handler.Expiry().Before(time.Now())
}

// 执行监听 conn 连接、异常等操作
func (handler *connectionHandler) ConnectionServer(done func()) {
	defer func() {
		// 通知连接池已完成
		done()
	}()
	logger.Infof("[connection_handler] -> begin Connection(%s) Handler Server", handler.key)
	defer logger.Infof("[connection_handler] -> stop Connection(%s) Handler Server", handler.key)

	// 开始连接服务
	// 开始读取, 返回只读的信息和异常 chan
	message, readError := handler.readConnection()

	for {
		select {
		case <-handler.cancel:
			return
		case <-handler.exTimer.C:
			if handler.Expiry().IsZero() {
				// handler expiryTime is zero only when TTL = 0 (unlimited handler)
				// so we must not get here with zero expiryTime
				logger.Error("[connection_handler] -> fires expiry timer with ZERO expiryTime")
			}
			select {
			case <-handler.cancel:
				return
			case handler.handleError <- fmt.Errorf("connection expired"):
				logger.Info("[connection_handler] -> connection expiry error passed up")
			}
		case msg, ok := <-message:
			if !ok {
				return
			}

			// pass up
			select {
			case <-handler.cancel:
				return
			case handler.output <- msg:
				logger.Debugf("[connection_handle:%s] -> SIP message passed up [connection_pool]", handler.Connection().Network())
			}

			if !handler.Expiry().IsZero() {
				handler.expiry = time.Now().Add(handler.ttl)
				handler.exTimer.Reset(handler.ttl)
			}
		case err, ok := <-readError:
			if !ok {
				return
			}
			select {
			case <-handler.cancel:
				return
			case handler.handleError <- err:
				logger.Info("[connection_handler] -> error passed up")
			}
		}

	}
	//handler.pipeOutputs(message, errs)

}

func (handler *connectionHandler) readConnection() (<-chan sip.Message, <-chan error) {
	message := make(chan sip.Message)
	readError := make(chan error)
	// 判断协议
	streamed := handler.Connection().Streamed()
	// 创建解析器 todo
	prs := sip.NewParser(message, readError, streamed)
	//prs := parser.NewParser(message, readError, streamed)

	// 开启 goroutine 读取
	go func() {
		defer func() {
			// 关闭连接
			handler.Close()
			// 停止解析
			prs.Stop()
			// 关闭通道
			close(message)
			close(readError)
		}()

		buf := make([]byte, BufferSize)
		var (
			num int
			err error
		)

		// wait for data
		for {
			if streamed {
				num, err = handler.Connection().Read(buf)
			} else {
				num, _, err = handler.Connection().ReadFrom(buf)
			}

			if err != nil {
				// if we get timeout error just go further and try read on the next iteration
				var netErr net.Error
				if errors.As(err, &netErr) {
					if netErr.Timeout() || netErr.Temporary() {
						logger.Warnf("[connection_handler] -> connection timeout or temporary unavailable, sleep by %s", NetErrRetryTime)
						time.Sleep(NetErrRetryTime)
						continue
					}
				}

				// broken or closed connection
				// so send error and exit
				select {
				case <-handler.cancel:
				case readError <- err:
				}

				return
			}

			data := buf[:num]

			// skip empty udp packets
			if len(bytes.Trim(data, "\x00")) == 0 {
				logger.Infof("[connection_handler] -> skip empty data: %#v", data)
				continue
			}

			// parse received data
			if _, err := prs.Write(append([]byte{}, buf[:num]...)); err != nil {
				select {
				case <-handler.cancel:
					return
				case readError <- err:
				}
			}

			//select {
			//case <-handler.cancel:
			//	return
			//case message <- sip.Message(data):
			//	logger.Info("[connection_handler] -> parse received data")
			//}

		}

	}()

	return message, readError
}

// 执行释放资源、关闭等操作
func (handler *connectionHandler) Close() {
	select {
	case <-handler.cancel:
		return
	default:
	}

	handler.cancelOnce.Do(func() {
		close(handler.cancel)

		if err := handler.Connection().Close(); err != nil {
			logger.Errorf("[connection_handler] -> connection close failed: %s", err)
		}
	})
}
