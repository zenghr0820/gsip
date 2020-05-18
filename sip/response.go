package sip

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	uuid "github.com/satori/go.uuid"
)

// Response RFC 3261 - 7.2.
type Response interface {
	Message
	StatusCode() StatusCode
	SetStatusCode(code StatusCode)
	Reason() string
	SetReason(reason string)
	// Previous returns previous provisional responses
	// 返回先前的临时响应
	Previous() []Response
	SetPrevious(responses []Response)
	/* Common helpers */
	IsProvisional() bool
	IsSuccess() bool
	IsRedirection() bool
	IsClientError() bool
	IsServerError() bool
	IsGlobalError() bool

	// 创建 invite 2xx响应 对应的 ack 请求
	CreateAck() Request
}

type response struct {
	message
	status   StatusCode
	reason   string
	previous []Response
}

func NewResponse(
	messID MessageID,
	sipVersion string,
	statusCode StatusCode,
	reason string,
	hdrs []Header,
	body string,
) Response {
	res := new(response)
	if messID == "" {
		res.messID = MessageID(uuid.Must(uuid.NewV4(), nil).String())
	} else {
		res.messID = messID
	}
	res.startLine = res.StartLine
	res.SetSipVersion(sipVersion)
	res.headers = newHeaders(hdrs)
	res.SetStatusCode(statusCode)
	res.SetReason(reason)

	if strings.TrimSpace(body) != "" {
		res.SetBody(body, true)
	}

	return res
}

func (res *response) Short() string {
	if res == nil {
		return "<nil>"
	}

	return fmt.Sprintf("sip.Response<%s>", res.messID)
}

func (res *response) StatusCode() StatusCode {
	return res.status
}
func (res *response) SetStatusCode(code StatusCode) {
	res.status = code
	res.reason = StatusText(code)
}

func (res *response) Reason() string {
	return res.reason
}
func (res *response) SetReason(reason string) {
	res.reason = reason
}

func (res *response) Previous() []Response {
	return res.previous
}

func (res *response) SetPrevious(responses []Response) {
	res.previous = responses
}

// StartLine returns Response Status Line - RFC 2361 7.2.
func (res *response) StartLine() string {
	var buffer bytes.Buffer

	// Every SIP response starts with a Status Line - RFC 2361 7.2.
	buffer.WriteString(
		fmt.Sprintf(
			"%s %d %s",
			res.SipVersion(),
			res.StatusCode(),
			res.Reason(),
		),
	)

	return buffer.String()
}

func (res *response) Copy() Message {
	return NewResponse(
		"",
		res.SipVersion(),
		res.StatusCode(),
		res.Reason(),
		res.headers.CloneHeaders(),
		res.Body(),
	)
}

func (res *response) IsProvisional() bool {
	return res.StatusCode() < 200
}

func (res *response) IsSuccess() bool {
	return res.StatusCode() >= 200 && res.StatusCode() < 300
}

func (res *response) IsRedirection() bool {
	return res.StatusCode() >= 300 && res.StatusCode() < 400
}

func (res *response) IsClientError() bool {
	return res.StatusCode() >= 400 && res.StatusCode() < 500
}

func (res *response) IsServerError() bool {
	return res.StatusCode() >= 500 && res.StatusCode() < 600
}

func (res *response) IsGlobalError() bool {
	return res.StatusCode() >= 600
}

func (res *response) IsAck() bool {
	if seq, ok := res.CSeq(); ok {
		return seq.MethodName == ACK
	}
	return false
}

func (res *response) IsCancel() bool {
	if seq, ok := res.CSeq(); ok {
		return seq.MethodName == CANCEL
	}
	return false
}

// RFC 3261 - 8.2.6
func NewResponseFromRequest(
	resID MessageID,
	req Request,
	statusCode StatusCode,
	reason string,
	body string,
) Response {
	res := NewResponse(
		resID,
		req.SipVersion(),
		statusCode,
		reason,
		[]Header{},
		"",
	)

	CopyHeaders("Record-Route", req, res)
	CopyHeaders("Via", req, res)
	CopyHeaders("From", req, res)
	CopyHeaders("To", req, res)
	CopyHeaders("Call-ID", req, res)
	CopyHeaders("CSeq", req, res)

	if statusCode == 100 {
		CopyHeaders("Timestamp", req, res)
	}

	res.SetSource(req.Destination())
	res.SetDestination(req.Source())

	if len(body) > 0 {
		res.SetBody(body, true)
	}

	return res
}

func (res *response) Source() string {
	return res.src
}

func (res *response) Destination() string {
	if res.dest != "" {
		return res.dest
	}

	viaHop, ok := res.ViaHop()
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

	if rport, ok := viaHop.Params.Get("rport"); ok && rport.String() != "" {
		p, _ := strconv.Atoi(rport.String())
		port = Port(uint16(p))
	} else if viaHop.Port != nil {
		port = *viaHop.Port
	} else {
		port = DefaultPort(res.Transport())
	}

	return fmt.Sprintf("%v:%v", host, port)
}

func (res *response) CreateAck() Request {
	remoteAddr := res.Source()
	ackRequest := CreateSimpleRequest(ACK, remoteAddr)
	ackRequest.SetSipVersion(res.SipVersion())

	CopyHeaders("Via", res, ackRequest)
	viaHop, _ := ackRequest.ViaHop()
	// update branch, 2xx ACK is separate Tx
	viaHop.Params.Add("branch", String{Str: GenerateBranch()})

	if len(res.GetHeaders("Route")) > 0 {
		CopyHeaders("Route", res, ackRequest)
	} else {
		for _, h := range res.GetHeaders("Record-Route") {
			uris := make([]Uri, 0)
			for _, u := range h.(*RecordRouteHeader).Addresses {
				uris = append(uris, u.Copy())
			}
			ackRequest.AddHeader(&RouteHeader{
				Addresses: uris,
			})
		}
	}

	CopyHeaders("From", res, ackRequest)
	CopyHeaders("To", res, ackRequest)
	CopyHeaders("Call-ID", res, ackRequest)
	CopyHeaders("CSeq", res, ackRequest)
	cSeq, _ := ackRequest.CSeq()
	cSeq.MethodName = ACK

	return ackRequest
}

func CopyResponse(res Response) Response {
	hdrs := make([]Header, 0)
	for _, header := range res.Headers() {
		hdrs = append(hdrs, header.Copy())
	}

	newRes := NewResponse(
		res.MessageID(),
		res.SipVersion(),
		res.StatusCode(),
		res.Reason(),
		hdrs,
		res.Body(),
	)
	newRes.SetPrevious(res.Previous())

	return newRes
}
