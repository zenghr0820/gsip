package transport

import (
	"fmt"
	"net"
	"sync"

	"github.com/zenghr0820/gsip/logger"
)

/**
实例化监听池：
	receiveConnection: 接收连接池数据的 chan
	receiveError: 接收连接池异常的 chan
	notifyCancel：通知连接池关闭的 chan
*/
func CreateListenerPool(
	receiveConnection chan<- Connection,
	receiveError chan<- error,
	notifyCancel <-chan struct{},
) ListenerPool {

	pool := &listenerPool{
		handleMap:        make(map[ListenerKey]ListenerHandler),
		output:           receiveConnection,
		poolError:        receiveError,
		listenConnection: make(chan Connection),
		listenError:      make(chan error),
		cancel:           make(chan struct{}),
		done:             make(chan struct{}),
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
type ListenerPool interface {
	Put(key ListenerKey, listener net.Listener) error
	Get(key ListenerKey) (net.Listener, error)
	All() []net.Listener
	Del(key ListenerKey) error
	DelAll() error
	Length() int
	Done() <-chan struct{}
}

// 连接池实现
type listenerPool struct {
	// 连接 - 处理服务 对应关系
	handleMap map[ListenerKey]ListenerHandler
	// 传递输出数据
	output chan<- Connection
	// 传递异常
	poolError chan<- error
	// 监听传递给连接池的数据
	listenConnection chan Connection
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
func (pool *listenerPool) ListenHandlers() {
	defer func() {
		pool.release() // 释放资源
		close(pool.done)
	}()

	logger.Info("[listener_pool] -> begin ListenHandlers")
	defer logger.Info("[listener_pool] -> stop ListenHandlers")

	for {
		select {
		case <-pool.cancel:
			return
		case connection, ok := <-pool.listenConnection:
			if !ok {
				return
			}
			if connection == nil {
				continue
			}
			logger.Debug("[listener_pool] listenConnection -> ", connection)

			// 往上层传递
			select {
			case <-pool.cancel:
				return
			case pool.output <- connection:
				logger.Info("[listener_pool] -> SIP message passed up [transport_layer]")

				continue
			}
		case err, ok := <-pool.listenError:
			if !ok {
				return
			}
			if err == nil {
				continue
			}
			logger.Debug("[listener_pool] listenError -> ", err)

			// 往上层传递
			select {
			case <-pool.cancel:
				return
			case pool.poolError <- err:
				logger.Info("[listener_pool] -> error passed up [transport_layer]")

				continue
			}
		}
	}
}

// 释放资源
func (pool *listenerPool) release() {
	logger.Info("[listener_pool] -> release resources")
	// 释放资源
	if err := pool.delAll(); err != nil {
		logger.Error("pool Close error: ", err)
	}

	pool.handleWg.Wait()
	// stop serveHandlers goroutine
	close(pool.listenConnection)
	close(pool.listenError)
}

func (pool *listenerPool) Done() <-chan struct{} {
	return pool.done
}

func (pool *listenerPool) Put(key ListenerKey, listener net.Listener) error {
	select {
	case <-pool.cancel:
		return fmt.Errorf("[Put listener] -> listener pool closed")
	default:
	}

	if key == "" {
		return fmt.Errorf("[Put listener] -> empty listener key")
	}

	if err := pool.put(key, listener); err != nil {
		return err
	}
	return nil
}

func (pool *listenerPool) put(key ListenerKey, listener net.Listener) error {
	if _, err := pool.get(key); err == nil {
		return fmt.Errorf("key %s already exists in the pool", key)
	}

	// 创建连接服务
	handle := CreateListenerHandler(key, listener, pool.listenConnection, pool.listenError)

	// 加锁
	pool.mu.Lock()
	pool.handleMap[key] = handle
	pool.mu.Unlock()

	pool.handleWg.Add(1)
	go handle.ListenerServer(pool.handleWg.Done)
	// 维护一个 goroutine 用于监听 连接服务传递的数据
	//go func() {
	//
	//}()

	return nil
}

func (pool *listenerPool) Get(key ListenerKey) (net.Listener, error) {
	select {
	case <-pool.cancel:
		return nil, fmt.Errorf("[Get listener] -> listener pool closed")
	default:
	}

	if key == "" {
		return nil, fmt.Errorf("[Get listener] -> empty listener key")
	}

	handle, err := pool.get(key)
	if handle == nil {
		return nil, err
	}
	return handle.Listener(), err
}

func (pool *listenerPool) get(key ListenerKey) (ListenerHandler, error) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	if handler, ok := pool.handleMap[key]; ok {
		return handler, nil
	}

	return nil, fmt.Errorf("listener %s not found in the pool", key)
}

func (pool *listenerPool) All() []net.Listener {
	select {
	case <-pool.cancel:
		return []net.Listener{}
	default:
	}

	return pool.all()
}

func (pool *listenerPool) all() []net.Listener {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	listeners := make([]net.Listener, len(pool.handleMap))
	for _, value := range pool.handleMap {
		listeners = append(listeners, value.Listener())
	}
	return listeners
}

func (pool *listenerPool) allKeys() []ListenerKey {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	keys := make([]ListenerKey, len(pool.handleMap))
	for k := range pool.handleMap {
		keys = append(keys, k)
	}
	return keys
}

func (pool *listenerPool) Del(key ListenerKey) error {
	select {
	case <-pool.cancel:
		return fmt.Errorf("[Del listener] -> listener pool closed")
	default:
	}

	if key == "" {
		return fmt.Errorf("[Del listener] -> empty listener key")
	}

	if err := pool.del(key); err != nil {
		return err
	}
	return nil
}

func (pool *listenerPool) del(key ListenerKey) error {
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

func (pool *listenerPool) DelAll() error {
	select {
	case <-pool.cancel:
		return fmt.Errorf("[DelAll listener] -> listener pool closed")
	default:
	}

	return pool.delAll()
}

func (pool *listenerPool) delAll() error {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	for _, value := range pool.handleMap {
		// 关闭 连接服务
		value.Close()
	}
	pool.handleMap = make(map[ListenerKey]ListenerHandler)
	return nil
}

func (pool *listenerPool) Length() int {
	return len(pool.handleMap)
}
