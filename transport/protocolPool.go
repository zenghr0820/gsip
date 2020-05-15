package transport

import (
	"sync"
)

// Thread-safe protocols pool.
// 线程安全协议池
type protocolPool struct {
	protocols map[protocolKey]Protocol
	mu        sync.RWMutex
}

func createProtocolPool() *protocolPool {
	return &protocolPool{
		protocols: make(map[protocolKey]Protocol),
	}
}

func (protocolPool *protocolPool) put(key protocolKey, protocol Protocol) {
	protocolPool.mu.Lock()
	protocolPool.protocols[protocolKey(key.String())] = protocol
	protocolPool.mu.Unlock()
}

func (protocolPool *protocolPool) get(key protocolKey) (Protocol, bool) {
	protocolPool.mu.RLock()
	defer protocolPool.mu.RUnlock()
	protocol, ok := protocolPool.protocols[protocolKey(key.String())]
	return protocol, ok
}

func (protocolPool *protocolPool) del(key protocolKey) bool {
	if _, ok := protocolPool.get(key); !ok {
		return false
	}
	protocolPool.mu.Lock()
	defer protocolPool.mu.Unlock()
	delete(protocolPool.protocols, protocolKey(key.String()))
	return true
}

func (protocolPool *protocolPool) all() []Protocol {
	all := make([]Protocol, 0)
	protocolPool.mu.RLock()
	defer protocolPool.mu.RUnlock()
	for _, protocol := range protocolPool.protocols {
		all = append(all, protocol)
	}

	return all
}
