package transport

import (
	"fmt"
	"sync"
	"time"

	"github.com/zenghr0820/gsip/logger"
	"github.com/zenghr0820/gsip/sip"
)

/**
实例化连接池：
	receiveMessage: 接收连接池数据的 chan
	receiveError: 接收连接池异常的 chan
	notifyCancel：通知连接池关闭的 chan
*/
func CreateConnectionPool(
	receiveMessage chan<- sip.Message,
	receiveError chan<- error,
	notifyCancel <-chan struct{},
) ConnectionPool {

	pool := &connectionPool{
		handleMap:     make(map[ConnectionKey]ConnectionHandler),
		output:        receiveMessage,
		poolError:     receiveError,
		listenMessage: make(chan sip.Message),
		listenError:   make(chan error),
		cancel:        make(chan struct{}),
		done:          make(chan struct{}),
	}

	// 启动一个 goroutine 来监听关闭通知
	go func() {
		<-notifyCancel
		close(pool.cancel)
	}()
	// 启动一个 goroutine 来监听连接池中的数据
	go pool.ListenHandlers()

	return pool
}

// 连接池定义
type ConnectionPool interface {
	Put(connection Connection, ttl time.Duration) error
	Get(key ConnectionKey) (Connection, error)
	All() []Connection
	Del(key ConnectionKey) error
	DelAll() error
	Length() int
	Done() <-chan struct{}
}

// 连接池实现
type connectionPool struct {
	// 连接 - 处理服务 对应关系
	handleMap map[ConnectionKey]ConnectionHandler
	// 传递输出数据
	output chan<- sip.Message
	// 传递异常
	poolError chan<- error
	// 监听传递给连接池的数据
	listenMessage chan sip.Message
	// 传递传递给连接池的异常
	listenError chan error
	// 等待所有 连接处理服务完成
	handleWg sync.WaitGroup
	mu       sync.RWMutex
	// 用于通知是否关闭
	cancel chan struct{}
	// 是否完成关闭
	done chan struct{}
}

// 监听 连接服务 传递的信息和异常
func (pool *connectionPool) ListenHandlers() {
	defer func() {
		pool.release() // 释放资源
		close(pool.done)
	}()

	logger.Info("[connection_pool] -> begin listen connectionHandler")
	defer logger.Info("[connection_pool] -> stop listen connectionHandler")

	for {
		select {
		case <-pool.cancel:
			return
		case message, ok := <-pool.listenMessage:
			if !ok {
				return
			}
			if message == nil {
				continue
			}
			logger.Debug("[connection_pool] listenMessage -> ", message)

			// 往上层传递
			select {
			case <-pool.cancel:
				return
			case pool.output <- message:
				logger.Debug("[connection_pool] -> SIP message passed up [transport_layer]")

				continue
			}
		case err, ok := <-pool.listenError:
			if !ok {
				return
			}
			if err == nil {
				continue
			}
			logger.Debug("[connection_pool] listenError -> ", err)

			// 往上层传递
			select {
			case <-pool.cancel:
				return
			case pool.poolError <- err:
				logger.Info("[connection_pool] -> error passed up [transport_layer]")

				continue
			}
		}
	}
}

// 释放资源
func (pool *connectionPool) release() {
	logger.Info("[connection_pool] -> release resources")
	// 释放资源
	if err := pool.delAll(); err != nil {
		logger.Error("pool Close error: ", err)
	}

	pool.handleWg.Wait()
	// stop serveHandlers goroutine
	close(pool.listenMessage)
	close(pool.listenError)
}

func (pool *connectionPool) Done() <-chan struct{} {
	return pool.done
}

func (pool *connectionPool) Put(connection Connection, ttl time.Duration) error {
	select {
	case <-pool.cancel:
		return fmt.Errorf("[Put connection] -> connection pool closed")
	default:
	}

	key := connection.Key()
	if key == "" {
		return fmt.Errorf("[Put connection] -> empty connection key")
	}

	if err := pool.put(key, connection, ttl); err != nil {
		return err
	}
	return nil
}

func (pool *connectionPool) put(key ConnectionKey, connection Connection, ttl time.Duration) error {
	if _, err := pool.get(key); err == nil {
		return fmt.Errorf("key %s already exists in the pool", key)
	}

	// 创建连接服务
	handle := CreateConnectionHandler(connection, ttl, pool.listenMessage, pool.listenError)

	// 加锁
	pool.mu.Lock()
	pool.handleMap[key] = handle
	pool.mu.Unlock()

	pool.handleWg.Add(1)
	go handle.ConnectionServer(pool.handleWg.Done)
	// 维护一个 goroutine 用于监听 连接服务传递的数据
	//go func() {
	//
	//}()

	return nil
}

func (pool *connectionPool) Get(key ConnectionKey) (Connection, error) {
	select {
	case <-pool.cancel:
		return nil, fmt.Errorf("[Get connection] -> connection pool closed")
	default:
	}

	if key == "" {
		return nil, fmt.Errorf("[Get connection] -> empty connection key")
	}

	handle, err := pool.get(key)
	if handle == nil {
		return nil, err
	}
	return handle.Connection(), err
}

func (pool *connectionPool) get(key ConnectionKey) (ConnectionHandler, error) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	if handler, ok := pool.handleMap[key]; ok {
		return handler, nil
	}

	return nil, fmt.Errorf("connection %s not found in the pool", key)
}

func (pool *connectionPool) All() []Connection {
	select {
	case <-pool.cancel:
		return []Connection{}
	default:
	}

	return pool.all()
}

func (pool *connectionPool) all() []Connection {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	connections := make([]Connection, len(pool.handleMap))
	for _, value := range pool.handleMap {
		connections = append(connections, value.Connection())
	}
	return connections
}

func (pool *connectionPool) allKeys() []ConnectionKey {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	keys := make([]ConnectionKey, len(pool.handleMap))
	for k := range pool.handleMap {
		keys = append(keys, k)
	}
	return keys
}

func (pool *connectionPool) Del(key ConnectionKey) error {
	select {
	case <-pool.cancel:
		return fmt.Errorf("[Del connection] -> connection pool closed")
	default:
	}

	if key == "" {
		return fmt.Errorf("[Del connection] -> empty connection key")
	}

	if err := pool.del(key); err != nil {
		return err
	}
	return nil
}

func (pool *connectionPool) del(key ConnectionKey) error {
	handler, err := pool.get(key)
	if err != nil {
		return fmt.Errorf("key %s already exists in the pool", key)
	}

	// 关闭 连接服务
	handler.Close()

	pool.mu.Lock()
	// 删除
	delete(pool.handleMap, key)
	pool.mu.Unlock()

	return nil
}

func (pool *connectionPool) DelAll() error {
	select {
	case <-pool.cancel:
		return fmt.Errorf("[DelAll connection] -> connection pool closed")
	default:
	}

	return pool.delAll()
}

func (pool *connectionPool) delAll() error {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	for _, value := range pool.handleMap {
		// 关闭 连接服务
		value.Close()
	}
	pool.handleMap = make(map[ConnectionKey]ConnectionHandler)
	return nil
}

func (pool *connectionPool) Length() int {
	return len(pool.handleMap)
}
