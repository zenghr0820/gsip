package sip

import "sync"

func CreateSession() Session {
	return &session{
		store: make(map[string]interface{}),
		mu:    new(sync.RWMutex),
	}
}

// Session 定义, 用于存储自定义数据
type Session interface {
	GetAttribute(key string) interface{}
	SetAttribute(key string, value interface{})
	DelAttribute(key string)
}

type session struct {
	store map[string]interface{}
	// 锁
	mu *sync.RWMutex
}

func (s *session) GetAttribute(key string) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.store[key]
}

func (s *session) SetAttribute(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.store[key] = value
}

func (s *session) DelAttribute(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 删除
	delete(s.store, key)
}
