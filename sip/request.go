package sip

import (
	"bytes"
	"fmt"
	"net"
	"strconv"

	uuid "github.com/satori/go.uuid"
	"github.com/zenghr0820/gsip/logger"
	"github.com/zenghr0820/gsip/utils"
)

// Request RFC 3261 - 7.1.
type Request interface {
	Message
	Method() RequestMethod
	SetMethod(method RequestMethod)
	Recipient() Uri
	SetRecipient(recipient Uri)
	/* Common Helpers */
	IsInvite() bool
	// 创建响应函数
	CreateResponse(statusCode StatusCode) Response
	// 创建响应函数 参数 reason
	CreateResponseReason(statusCode StatusCode, reason string) Response
	// 添加认证信息
	AddAuthHeader(response Response, username, password string) error
}

type request struct {
	message
	method    RequestMethod
	recipient Uri
}

func CreateRequest(method RequestMethod, remoteAddr string, from, to Uri) Request {
	req := new(request)
	req.messID = MessageID(uuid.Must(uuid.NewV4(), nil).String())
	// 生成 via from to Call-ID Cseq Max-Forwards
	via := DefaultViaHeader()
	// 解析 from to
	fromHeader := &FromHeader{
		DisplayName: nil,
		Address:     from,
		Params:      NewParams(),
	}

	toHeader := &ToHeader{
		DisplayName: nil,
		Address:     to,
		Params:      NewParams(),
	}
	callId := DefaultCallID()
	cSeq := DefaultCSeq()
	maxForwards := DefaultMaxForwards()
	headerList := []Header{via, fromHeader, toHeader, callId, cSeq, maxForwards}
	req.headers = newHeaders(headerList)
	req.startLine = req.StartLine

	req.SetSipVersion(SipVersion)
	req.SetMethod(method)
	req.SetDestination(remoteAddr)

	return req
}

func CreateSimpleRequest(method RequestMethod, remoteAddr string) Request {
	req := new(request)
	req.messID = MessageID(uuid.Must(uuid.NewV4(), nil).String())
	req.headers = newHeaders([]Header{})
	req.startLine = req.StartLine

	req.SetSipVersion(SipVersion)
	req.SetMethod(method)
	req.SetDestination(remoteAddr)

	return req
}

//func newRequest() Request {
//	req := new(request)
//	req.messID = MessageID(uuid.Must(uuid.NewV4(), nil).String())
//	req.headers = newHeaders([]Header{})
//	req.startLine = req.StartLine
//	return req
//}

func (req *request) Short() string {
	if req == nil {
		return "<nil>"
	}

	return fmt.Sprintf("sip.Request<%s>", req.messID)
}

func (req *request) Method() RequestMethod {
	return req.method
}
func (req *request) SetMethod(method RequestMethod) {
	req.method = method
	if cSeq, ok := req.CSeq(); ok {
		cSeq.MethodName = method
	}
}

func (req *request) Recipient() Uri {
	return req.recipient
}
func (req *request) SetRecipient(recipient Uri) {
	req.recipient = recipient
}

// StartLine returns Request Line - RFC 2361 7.1.
func (req *request) StartLine() string {
	var buffer bytes.Buffer

	// Every SIP request starts with a Request Line - RFC 2361 7.1.
	var recipient Uri = &SipUri{
		FIsEncrypted: false,
		FUser:        nil,
		FPassword:    nil,
		FUriParams:   nil,
		FHeaders:     nil,
	}

	if req.method == REGISTER {
		host, port, err := net.SplitHostPort(req.Destination())
		if err != nil {
			logger.Error(err)
		}
		domain := Addr{
			Host: host,
			Port: nil,
		}
		if i, err := strconv.Atoi(port); err == nil {
			port := Port(i)
			domain.Port = &port
		}

		recipient.SetDomain(domain)
	} else {
		to, ok := req.To()
		if ok {
			recipient = to.Address.Copy()
			recipient.SetUriParams(nil)
			recipient.SetHeaders(nil)
		}
	}
	req.SetRecipient(recipient)

	logger.Info("recipient = ", recipient)

	buffer.WriteString(
		fmt.Sprintf(
			"%s %s %s",
			string(req.method),
			recipient,
			req.SipVersion(),
		),
	)

	return buffer.String()
}

func (req *request) Copy() Message {
	newReq := CreateSimpleRequest(req.method, req.Destination())
	newReq.SetSipVersion(req.SipVersion())
	newReq.SetRecipient(req.Recipient().Copy())
	for _, header := range req.headers.CloneHeaders() {
		newReq.AddHeader(header)
	}

	newReq.SetBody(req.Body(), true)
	return newReq
}

func (req *request) IsInvite() bool {
	return req.Method() == INVITE
}

func (req *request) IsAck() bool {
	return req.Method() == ACK
}

func (req *request) IsCancel() bool {
	return req.Method() == CANCEL
}

func (req *request) Source() string {
	if req.src != "" {
		return req.src
	}

	viaHop, ok := req.ViaHop()
	if !ok {
		return ""
	}

	var (
		host string
		port Port
	)

	if received, ok := viaHop.Params.Get("received"); ok && received.String() != "" {
		host = received.String()
	} else {
		host = viaHop.Host
	}

	if rport, ok := viaHop.Params.Get("rport"); ok && rport != nil && rport.String() != "" {
		p, _ := strconv.Atoi(rport.String())
		port = Port(uint16(p))
	} else if viaHop.Port != nil {
		port = *viaHop.Port
	} else {
		port = DefaultPort(req.Transport())
	}

	return fmt.Sprintf("%v:%v", host, port)
}

func (req *request) Destination() string {
	if req.dest != "" {
		return req.dest
	}

	var uri *SipUri
	if hdrs := req.GetHeaders("Route"); len(hdrs) > 0 {
		routeHeader := hdrs[0].(*RouteHeader)
		if len(routeHeader.Addresses) > 0 {
			uri = routeHeader.Addresses[0].(*SipUri)
		}
	}
	if uri == nil {
		if u, ok := req.Recipient().(*SipUri); ok {
			uri = u
		} else {
			return ""
		}
	}

	host := uri.FDomain.Host
	var port Port
	if uri.FDomain.Port == nil {
		port = DefaultPort(req.Transport())
	} else {
		port = *uri.FDomain.Port
	}

	return fmt.Sprintf("%v:%v", host, port)
}

// 创建请求对应的响应 RFC 3261 - 8.2.6
func (req *request) CreateResponse(statusCode StatusCode) Response {
	res := new(response)

	res.messID = MessageID(uuid.Must(uuid.NewV4(), nil).String()) // 唯一 ID
	res.startLine = res.StartLine                                 // 开始行
	res.SetSipVersion(req.sipVersion)                             // sip 版本
	res.SetStatusCode(statusCode)                                 // 响应状态码
	// 复制头部参数
	res.headers = newHeaders([]Header{})
	res.body = req.body

	CopyHeaders("Record-Route", req, res)
	CopyHeaders("Via", req, res)
	CopyHeaders("From", req, res)
	CopyHeaders("To", req, res)
	CopyHeaders("Call-ID", req, res)
	CopyHeaders("CSeq", req, res)

	// RFC 3261 - 8.2.6.2 包头和 Tags
	// 当需要产生一个 100(Trying)应答的时候，所有对应请求中包头的 Timestamp 域必须也拷 贝到这个应答包头
	// 此外，UAS 还必须增加一个 Tag 到 To 头域 上(100(trying)应答 是一个例外
	if statusCode == 100 {
		CopyHeaders("Timestamp", req, res)
	} else if to, ok := res.To(); !ok || !to.Params.Has("tag") {
		to.Params.Add("tag", &String{utils.RandString(10, true)})
	}

	// 交换来源和目的地地址
	res.SetSource(req.Destination())
	res.SetDestination(req.Source())

	// 设置事务层
	//if tx := req.Transaction(); tx != nil {
	//	res.SetTransaction(tx)
	//}

	return res
}

func (req *request) CreateResponseReason(statusCode StatusCode, reason string) Response {
	res := new(response)

	res.messID = MessageID(uuid.Must(uuid.NewV4(), nil).String()) // 唯一 ID
	res.startLine = res.StartLine                                 // 开始行
	res.SetReason(reason)                                         // reason
	res.SetSipVersion(req.sipVersion)                             // sip 版本
	res.SetStatusCode(statusCode)                                 // 响应状态码
	// 复制头部参数
	res.headers = newHeaders([]Header{})
	res.body = req.body

	CopyHeaders("Record-Route", req, res)
	CopyHeaders("Via", req, res)
	CopyHeaders("From", req, res)
	CopyHeaders("To", req, res)
	CopyHeaders("Call-ID", req, res)
	CopyHeaders("CSeq", req, res)

	// RFC 3261 - 8.2.6.2 包头和 Tags
	// 当需要产生一个 100(Trying)应答的时候，所有对应请求中包头的 Timestamp 域必须也拷 贝到这个应答包头
	// 此外，UAS 还必须增加一个 Tag 到 To 头域 上(100(trying)应答 是一个例外
	if statusCode == 100 {
		CopyHeaders("Timestamp", req, res)
	} else if to, ok := res.To(); !ok || !to.Params.Has("tag") {
		to.Params.Add("tag", &String{utils.RandString(10, true)})
	}

	// 交换来源和目的地地址
	res.SetSource(req.Destination())
	res.SetDestination(req.Source())

	return res
}

// 给请求头部添加认证信息
func (req *request) AddAuthHeader(response Response, username, password string) error {
	auth := &DefaultAuthorized{
		User:     MaybeString(String{Str: username}),
		Password: MaybeString(String{Str: password}),
	}

	return auth.AddAuthInfo(req, response)
}

// NewAckForInvite creates ACK request for 2xx INVITE
// https://tools.ietf.org/html/rfc3261#section-13.2.2.4
