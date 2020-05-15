package callback

import (
	"errors"
	"sync"

	"github.com/zenghr0820/gsip/logger"
	"github.com/zenghr0820/gsip/sip"
)

var (
	DefaultCallback = NewCallback()
)

func NewCallback(opts ...Option) Callback {
	options := Options{}

	for _, o := range opts {
		o(&options)
	}

	return &callback{
		opts:            options,
		mu:              new(sync.RWMutex),
		requestHandlers: make(map[sip.RequestMethod]sip.RequestHandler),
		//responseHandlers: make(map[sip.ResponseMethod]sip.ResponseHandler),
	}
}

type callback struct {
	opts Options
	// 请求处理方法集合
	requestHandlers map[sip.RequestMethod]sip.RequestHandler
	// 响应请求处理方法集合
	//responseHandlers map[sip.ResponseMethod]sip.ResponseHandler
	responseHandler sip.ResponseHandler
	mu              *sync.RWMutex
}

func (c *callback) Init(opts ...Option) {
	for _, o := range opts {
		o(&c.opts)
	}
}

func (c *callback) Options() Options {
	return c.opts
}

func (c *callback) AddRequestHandle(method sip.RequestMethod, handler sip.RequestHandler) {
	c.mu.Lock()
	c.requestHandlers[method] = handler
	c.mu.Unlock()
}

//func (c *callback) AddResponseHandle(method sip.ResponseMethod, handler sip.ResponseHandler) {
//	c.mu.Lock()
//	c.responseHandlers[method] = handler
//	c.mu.Unlock()
//}

func (c *callback) SetRequestHandle(callback map[sip.RequestMethod]sip.RequestHandler) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if callback == nil {
		return errors.New("[requestHandle] -> Parameter exception")
	}
	c.requestHandlers = callback
	return nil
}

func (c *callback) SetResponseHandle(callback sip.ResponseHandler) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if callback == nil {
		return errors.New("[responseHandle] -> Parameter exception")
	}
	c.responseHandler = callback
	return nil
}

func (c *callback) GetRequestHandle(method sip.RequestMethod) (sip.RequestHandler, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	handler, ok := c.requestHandlers[method]
	return handler, ok
}

func (c *callback) GetResponseHandle() (sip.ResponseHandler, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.responseHandler != nil {
		return c.responseHandler, true
	}
	return nil, false
}

func (c *callback) DoRequest(request sip.Request, tx sip.ServerTransaction) error {
	handler, ok := c.GetRequestHandle(request.Method())
	if !ok {
		logger.Warnf("[requestHandle] -> SIP request handler not found")

		return &NotExitCallbackError{
			name: string(request.Method()),
		}
	}
	go handler(request, tx)

	return nil
}

func (c *callback) DoResponse(response sip.Response, tx sip.ClientTransaction) error {
	handler, ok := c.GetResponseHandle()

	if !ok {
		logger.Warnf("[responseHandle] -> SIP response handler not found")

		return &NotExitCallbackError{
			name: "response",
		}
	}
	go handler(response, tx)

	return nil
}

func (c *callback) String() string {
	return "callback"
}

// 返回实现的回调函数
func (c *callback) GetAllowedMethods() []sip.RequestMethod {
	var methods []sip.RequestMethod

	c.mu.RLock()
	for method := range c.requestHandlers {
		methods = append(methods, method)
	}
	c.mu.RUnlock()

	return methods
}
