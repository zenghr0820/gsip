package gsip

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/zenghr0820/gsip/callback"
	"github.com/zenghr0820/gsip/logger"
	"github.com/zenghr0820/gsip/sip"
	"github.com/zenghr0820/gsip/utils"
)

type service struct {
	opts Options

	close chan bool
	hwg   sync.WaitGroup
	once  sync.Once
}

func newService(opts ...Option) Service {
	service := new(service)
	service.opts = newOptions(opts...)
	// 开启 goroutine 监听 SIP 服务
	go service.start()
	return service
}

func (s *service) Listen(network string, listenAddr string) error {
	return s.opts.tp.Listen(network, listenAddr)
}

// 发送请求或者响应
func (s *service) Send(message sip.Message) (sip.Transaction, error) {
	select {
	case <-s.close:
		return nil, fmt.Errorf("[G.SIP] -> G.SIP Service Closed")
	default:
	}

	logger.Debug("[G.SIP] -> Start parsing information type ")
	s.autoFillMessageHeaderAndSend(message)
	return s.opts.tx.Send(message)
}

func (s *service) autoFillMessageHeaderAndSend(message sip.Message) {
	autoAppendMethods := map[sip.RequestMethod]bool{
		sip.INVITE:   true,
		sip.REGISTER: true,
		sip.OPTIONS:  true,
		sip.REFER:    true,
		sip.NOTIFY:   true,
	}

	var msgMethod sip.RequestMethod
	var statusCode sip.StatusCode
	switch m := message.(type) {
	case sip.Request:
		msgMethod = m.Method()
	case sip.Response:
		statusCode = m.StatusCode()
		if cseq, ok := m.CSeq(); ok && !m.IsProvisional() {
			msgMethod = cseq.MethodName
		}
	}

	if statusCode == sip.StatusMethodNotAllowed || len(msgMethod) > 0 {
		if _, ok := autoAppendMethods[msgMethod]; ok {
			hdrs := message.GetHeaders("Allow")
			if len(hdrs) == 0 {
				allow := make(sip.AllowHeader, 0)
				for _, method := range s.opts.Callback.GetAllowedMethods() {
					allow = append(allow, method)
				}

				message.AddHeader(allow)
			}

			hdrs = message.GetHeaders("Supported")
			if len(hdrs) == 0 {
				message.AddHeader(&sip.SupportedHeader{
					Options: []string{},
				})
			}
		}
	}

	if hdrs := message.GetHeaders("User-Agent"); len(hdrs) == 0 {
		userAgent := sip.UserAgentHeader("GSIP")
		message.AddHeader(&userAgent)
	}

	// from tag
	if from, ok := message.From(); ok && !from.Params.Has("tag") {
		from.Params.Add("tag", &sip.String{Str: utils.RandString(10, true)})
	}
}

func (s *service) Run() error {

	select {
	case <-s.close:
		return fmt.Errorf("[G.SIP] -> G.SIP Service Closed")
	}

}

func (s *service) Close() error {
	// 确保关闭方法只执行一次
	s.once.Do(func() {
		s.stop()
	})

	return nil
}

func (s *service) Options() Options {
	return s.opts
}

func (s *service) String() string {
	return "g.sip"
}

// 开始运行 SIP 服务
func (s *service) start() {
	defer func() {
		err := s.Close()
		if err != nil {
			logger.Error("[G.SIP] Close error： ", err)
		}
	}()

	for {
		select {
		case request, ok := <-s.opts.tx.Requests():
			if !ok {
				return
			}
			s.hwg.Add(1)
			go s.handleRequest(request)
		case res, ok := <-s.opts.tx.Responses():
			if !ok {
				return
			}
			s.hwg.Add(1)
			go s.handleResponse(res)
		case err, ok := <-s.opts.tx.Errors():
			if !ok {
				return
			}
			logger.Errorf("[G.SIP] -> received SIP transaction error: %s", err)
		case err, ok := <-s.opts.tp.Errors():
			if !ok {
				return
			}
			logger.Errorf("[G.SIP] -> received SIP transport error: %s", err)
		}
	}
}

// 停止服务
func (s *service) stop() {
	select {
	case <-s.close:
		return
	default:
	}

	// stop transaction layer
	s.opts.tx.Close()
	<-s.opts.tx.Done()
	// stop transport layer
	s.opts.tp.Close()
	<-s.opts.tp.Done()
	// wait for handlers
	s.hwg.Wait()
	// stop service
	close(s.close)
}

// 处理请求
func (s *service) handleRequest(request sip.Request) {
	defer s.hwg.Done()

	var tx sip.ServerTransaction
	if t := request.Transaction(); t != nil {
		tx = t.(sip.ServerTransaction)
	}

	err := s.opts.Callback.DoRequest(request, tx)
	// NotExitCallbackError
	var notExitCallbackError *callback.NotExitCallbackError
	if err != nil && errors.As(err, &notExitCallbackError) {
		logger.Warnf("[G.SIP] -> SIP %s request handler not found", request.Method())

		response := request.CreateResponseReason(sip.StatusMethodNotAllowed, "Method Not Allowed")
		if _, err := s.Send(response); err != nil {
			logger.Errorf("[G.SIP] -> Send '405 Method Not Allowed' failed: %s", err)
		}

		return
	}
}

// 处理响应
func (s *service) handleResponse(response sip.Response) {
	defer s.hwg.Done()
	var tx sip.ClientTransaction
	if t := response.Transaction(); t != nil {
		tx = t.(sip.ClientTransaction)
	}
	err := s.opts.Callback.DoResponse(response, tx)
	// NotExitCallbackError
	var notExitCallbackError *callback.NotExitCallbackError
	if err != nil && errors.As(err, &notExitCallbackError) {
		logger.Errorf("[G.SIP] -> SIP %s response handler not found", response.StatusCode())
	}
}

// 生成认证
func CreateAuthInfo(username, password string) *sip.DefaultAuthorized {
	auth := &sip.DefaultAuthorized{
		User:     sip.MaybeString(sip.String{Str: username}),
		Password: sip.MaybeString(sip.String{Str: password}),
	}

	return auth
}

/**
 * 生成请求
 *
 * @param method RequestMethod 请求类型
 * @param remoteAddr string remoteAddr 发送地址
 * @param from string from 头部参数
 * @param to string to 头部参数
 * @return Request 返回请求
 */
func (s *service) CreateRequest(method sip.RequestMethod, remoteAddr string, from, to sip.Uri) sip.Request {
	return sip.CreateRequest(method, remoteAddr, from, to)
}

func (s *service) CreateSimpleRequest(method sip.RequestMethod, remoteAddr string) sip.Request {
	return sip.CreateSimpleRequest(method, remoteAddr)
}

/**
 * 生成 Sip 地址
 *
 * @param user string 用户(账号:密码)：abc:123456
 * @param domain string SIP 域: ("127.0.0.1:5060")
 * @return sip.Uri 返回 Uri
 */
func (s *service) CreateSipUri(user string, domain string) sip.Uri {

	sipUri := &sip.SipUri{
		FIsEncrypted: false,
		FUser:        nil,
		FPassword:    nil,
		FDomain:      sip.Addr{},
		FUriParams:   sip.NewParams(),
		FHeaders:     sip.NewParams(),
	}

	userIdx := strings.Index(user, ":")
	if userIdx == -1 {
		sipUri.FUser = sip.String{Str: user}
	} else {
		sipUri.FUser = sip.String{Str: user[:userIdx]}
		sipUri.FPassword = sip.String{Str: user[userIdx+1:]}
	}

	domainIdx := strings.Index(domain, ":")
	if domainIdx == -1 {
		sipUri.FDomain.Host = domain
	} else {
		sipUri.FDomain.Host = domain[:domainIdx]
		if p, err := strconv.Atoi(domain[domainIdx+1:]); err == nil {
			port := sip.Port(p)
			sipUri.FDomain.Port = &port
		}

	}

	return sipUri
}
