package transaction

import (
	"sync"

	"github.com/zenghr0820/gsip/logger"
	"github.com/zenghr0820/gsip/sip"
)

// Thread-safe session pool.
// 线程安全会话池
type sessionPool struct {
	sessionList map[string]sip.Session

	mu sync.RWMutex
}

func createSessionPool() *sessionPool {
	return &sessionPool{
		sessionList: make(map[string]sip.Session),
	}
}

// 监听该 session 取消
func (store *sessionPool) listenSession(session sip.Session) {
	defer func() {
		store.drop(session.SessionId())
		logger.Debugf("[sessionPool] -> sessionStore[%s] deleted", session.SessionId())
	}()

	logger.Debug("[sessionPool] -> start serve session")
	defer logger.Debug("[sessionPool] -> stop serve session")

	for {
		select {
		case <-session.Done():
			return
		}
	}
}

func (store *sessionPool) put(key string, session sip.Session) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.sessionList[key] = session
	logger.Info("session add = ", session.SessionId())
	// 监听
	go store.listenSession(session)
}

func (store *sessionPool) get(key string) sip.Session {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.sessionList[key]
}

func (store *sessionPool) drop(key string) {
	if session := store.get(key); session != nil {
		session.Close()
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	delete(store.sessionList, key)
}

func (store *sessionPool) all() []sip.Session {
	all := make([]sip.Session, 0)
	store.mu.RLock()
	defer store.mu.RUnlock()
	for _, session := range store.sessionList {
		all = append(all, session)
	}

	return all
}
