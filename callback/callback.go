package callback

import (
	"github.com/zenghr0820/gsip/sip"
)

// 定义回调函数
type Callback interface {
	// Init the callback
	// 初始化 callback
	Init(opts ...Option)
	// callback 的配置选项
	Options() Options
	// 根据 Method 添加回调函数
	AddRequestHandle(method sip.RequestMethod, handler sip.RequestHandler)
	//AddResponseHandle(method sip.ResponseMethod, handler sip.ResponseHandler)
	// 设置
	SetRequestHandle(callback map[sip.RequestMethod]sip.RequestHandler) error
	SetResponseHandle(callback sip.ResponseHandler) error
	// 根据 Method 获取回调函数
	GetRequestHandle(method sip.RequestMethod) (sip.RequestHandler, bool)
	GetResponseHandle() (sip.ResponseHandler, bool)
	// 执行回调函数
	DoRequest(request sip.Request, tx sip.ServerTransaction) error
	DoResponse(response sip.Response, tx sip.ClientTransaction) error
	// 返回用户实现的函数
	GetAllowedMethods() []sip.RequestMethod

	String() string
}

// 定义异常
type NotExitCallbackError struct {
	name string // 函数对应名称
}

func (notExitError *NotExitCallbackError) Error() string {
	return "NotExitCallbackError: " + notExitError.name
}
