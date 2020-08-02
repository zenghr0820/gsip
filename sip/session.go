package sip

import (
	"sync"
	"sync/atomic"

	"github.com/zenghr0820/gsip/logger"
)

func CreateSession(key string) Session {
	return &session{
		key:       key,
		state:     0,
		store:     make(map[string]interface{}),
		callId:    "",
		from:      FromHeader{},
		to:        ToHeader{},
		seq:       1,
		mu:        new(sync.RWMutex),
		close:     make(chan struct{}),
		closeOnce: sync.Once{},
	}
}

type SessionKey string

func (key SessionKey) String() string {
	return string(key)
}

// Session 定义, 用于存储自定义数据
type Session interface {
	SessionId() string
	GetAttribute(key string) interface{}
	SetAttribute(key string, value interface{})
	DelAttribute(key string)
	Seq() uint32
	CallId() string
	SeqAtom()                                             // seq 自增 +1
	SaveMessage(msg Message)                              // 保存会话信息
	CreateRequest(method RequestMethod) (request Request) // 生成符合会话内的请求
	Close()                                               // 关闭 session
	Done() <-chan struct{}                                // session 是否已经销毁

}

type session struct {
	key       string // session 唯一标识(dialog-id)
	state     int    // 状态
	store     map[string]interface{}
	callId    CallID        // call-id
	from      FromHeader    // from
	to        ToHeader      // to
	seq       uint32        // seq
	mu        *sync.RWMutex // 锁
	close     chan struct{} // session 是否注销
	closeOnce sync.Once
}

func (s *session) SessionId() string {
	return s.key
}

func (s *session) SeqAtom() {
	atomic.AddUint32(&s.seq, 1)
}

func (s *session) Seq() uint32 {
	return s.seq
}

func (s *session) CallId() string {
	return string(s.callId)
}

// 销毁 session
func (s *session) Close() {
	select {
	case <-s.close:
		return
	default:
	}

	s.closeOnce.Do(func() {
		s.store = nil
		close(s.close)
		logger.Debugf("close session id: %s", s.SessionId())
	})
}

// 确认关闭操作是否完成
func (s *session) Done() <-chan struct{} {
	return s.close
}

// 保存会话信息
func (s *session) SaveMessage(msg Message) {
	// call-id
	if c := msg.CallID(); c != nil {
		if cHeader, ok := c.Copy().(*CallID); ok {
			s.callId = *cHeader
		}
	}
	// from
	if f := msg.From(); f != nil {
		if fromHeader, ok := f.Copy().(*FromHeader); ok {
			s.from = *fromHeader
		}
	}
	// to
	if t := msg.To(); t != nil {
		if toHeader, ok := t.Copy().(*ToHeader); ok {
			s.to = *toHeader
		}
	}
}

// 根据上下文填充请求
func (s *session) CreateRequest(method RequestMethod) (request Request) {
	request = CreateRequest(method, s.to.Address.Domain().String(), s.from.Address.Copy(), s.to.Address.Copy())
	// from
	request.From().Params = s.from.Params.Copy()
	// to
	request.To().Params = s.to.Params.Copy()
	// callId
	request.ReplaceHeader(s.callId.Copy())
	// seq
	request.CSeq().SeqNo = s.seq
	request.CSeq().MethodName = method
	return
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
