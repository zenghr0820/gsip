package sip

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/zenghr0820/gsip/utils"
)

/**
设置一些常用的类型
*/
const SipVersion string = "SIP/2.0"

// 请求类型
type RequestMethod string
type RequestHandler func(req Request, tx ServerTransaction)

// Determine if the given method equals some other given method.
// This is syntactic sugar for case insensitive equality checking.
func (method *RequestMethod) Equals(other *RequestMethod) bool {
	if method != nil && other != nil {
		return strings.EqualFold(string(*method), string(*other))
	} else {
		return method == other
	}
}

// 响应类型
type ResponseMethod string
type ResponseHandler func(req Response, tx ClientTransaction)

// It's nicer to avoid using raw strings to represent methods, so the following standard
// method names are defined here as constants for convenience.
const (
	INVITE    RequestMethod = "INVITE"
	ACK       RequestMethod = "ACK"
	CANCEL    RequestMethod = "CANCEL"
	MESSAGE   RequestMethod = "MESSAGE"
	BYE       RequestMethod = "BYE"
	REGISTER  RequestMethod = "REGISTER"
	OPTIONS   RequestMethod = "OPTIONS"
	SUBSCRIBE RequestMethod = "SUBSCRIBE"
	NOTIFY    RequestMethod = "NOTIFY"
	REFER     RequestMethod = "REFER"
	INFO      RequestMethod = "INFO"
)

const (
	PROVISIONAL ResponseMethod = "ProvisionalResponse_1XX"
	SUCCESS     ResponseMethod = "SuccessResponse_2XX"
	REDIRECT    ResponseMethod = "RedirectResponse_3XX"
	ERROR       ResponseMethod = "ErrorResponse_4xx"
)

const (
	DefaultProtocol      = "UDP"
	DefaultHost          = "0.0.0.0"
	DefaultUdpPort  Port = 5060
	DefaultTcpPort  Port = 5060
	DefaultTlsPort  Port = 5061
)

// 符合 RFC - 3261 Branch 的标识
const RFC3261BranchMagicCookie = "z9hG4bK"

// 端口
type Port uint16

func (port *Port) Copy() *Port {
	if port == nil {
		return nil
	}
	newPort := *port
	return &newPort
}

// 地址实体
type Addr struct {
	Host string
	Port *Port
}

func (trg Addr) String() string {
	if trg.Port != nil {
		return fmt.Sprintf("%v:%v", trg.Host, *trg.Port)
	}

	return trg.Host
}

func (trg *Addr) Addr() string {
	var (
		host string
		port Port
	)

	if strings.TrimSpace(trg.Host) != "" {
		host = trg.Host
	} else {
		host = DefaultHost
	}

	if trg.Port != nil {
		port = *trg.Port
	}

	return fmt.Sprintf("%v:%v", host, port)
}

func NewAddr(host string, port int) *Addr {
	cport := Port(port)

	return &Addr{Host: host, Port: &cport}
}

func NewTargetFromAddr(addr string) (*Addr, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	sipPort, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}
	return NewAddr(host, sipPort), nil
}

// 用默认值填补空值
func FillTargetHostAndPort(network string, target *Addr) *Addr {
	if strings.TrimSpace(target.Host) == "" {
		target.Host = DefaultHost
	}
	if target.Port == nil {
		p := DefaultPort(network)
		target.Port = &p
	}

	return target
}

// String 包装
type MaybeString interface {
	String() string
	Equals(other interface{}) bool
}

type String struct {
	Str string
}

func (str String) String() string {
	return str.Str
}

func (str String) Equals(other interface{}) bool {
	if v, ok := other.(String); ok {
		return str.Str == v.Str
	}

	return false
}

// DefaultPort returns protocol default port by network.
func DefaultPort(protocol string) Port {
	switch strings.ToLower(protocol) {
	case "tls":
		return DefaultTlsPort
	case "tcp":
		return DefaultTcpPort
	case "udp":
		return DefaultUdpPort
	default:
		return DefaultTcpPort
	}
}

// GenerateBranch returns random unique branch ID.
// 生成随机的 Branch Id
func GenerateBranch() string {
	return strings.Join([]string{
		RFC3261BranchMagicCookie,
		utils.RandString(32, false),
	}, "")
}
