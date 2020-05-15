package transport

import (
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"

	"github.com/zenghr0820/gsip/sip"
)

// 定义传输层的错误

// Transport error
type Error interface {
	net.Error
	// Network indicates network level errors
	Network() bool
}

func isNetwork(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	} else {
		return errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe)
	}
}
func isTimeout(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return false
}
func isTemporary(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Temporary()
	}
	return false
}
func isCanceled(err error) bool {
	var cancelErr sip.CancelError
	if errors.As(err, &cancelErr) {
		return cancelErr.Canceled()
	}
	return false
}
func isExpired(err error) bool {
	var expiryErr sip.ExpireError
	if errors.As(err, &expiryErr) {
		return expiryErr.Expired()
	}
	return false
}

// Connection level error.
// 连接级别错误
type ConnectionError struct {
	Err     error
	Op      string
	Net     string
	Source  string
	Dest    string
	ConnPtr string
}

func (err *ConnectionError) Unwrap() error   { return err.Err }
func (err *ConnectionError) Network() bool   { return isNetwork(err.Err) }
func (err *ConnectionError) Timeout() bool   { return isTimeout(err.Err) }
func (err *ConnectionError) Temporary() bool { return isTemporary(err.Err) }
func (err *ConnectionError) Error() string {
	if err == nil {
		return "<nil>"
	}

	fields := make(map[string]interface{})

	if err.Net != "" {
		fields["net"] = err.Net
	}
	if err.ConnPtr != "" {
		fields["connection_ptr"] = err.ConnPtr
	}
	if err.Source != "" {
		fields["source"] = err.Source
	}
	if err.Dest != "" {
		fields["destination"] = err.Dest
	}

	return fmt.Sprintf("transport.ConnectionError<%s> %s failed: %s", fields, err.Op, err.Err)
}

// 过期级别错误
type ExpireError string

func (err ExpireError) Network() bool   { return false }
func (err ExpireError) Timeout() bool   { return true }
func (err ExpireError) Temporary() bool { return false }
func (err ExpireError) Canceled() bool  { return false }
func (err ExpireError) Expired() bool   { return true }
func (err ExpireError) Error() string   { return "transport.ExpireError: " + string(err) }

// Net Protocol level error
// 网络协议错误
type ProtocolError struct {
	Err      error
	Op       string
	ProtoPtr string
}

func (err *ProtocolError) Unwrap() error   { return err.Err }
func (err *ProtocolError) Network() bool   { return isNetwork(err.Err) }
func (err *ProtocolError) Timeout() bool   { return isTimeout(err.Err) }
func (err *ProtocolError) Temporary() bool { return isTemporary(err.Err) }
func (err *ProtocolError) Error() string {
	if err == nil {
		return "<nil>"
	}

	fields := make(map[string]interface{})

	if err.ProtoPtr != "" {
		fields["protocol_ptr"] = err.ProtoPtr
	}

	return fmt.Sprintf("transport.ProtocolError<%s> %s failed: %s", fields, err.Op, err.Err)
}

// 连接服务错误
type ConnectionHandlerError struct {
	Err        error
	Key        ConnectionKey
	HandlerPtr string
	Net        string
	LAddr      string
	RAddr      string
}

func (err *ConnectionHandlerError) Unwrap() error   { return err.Err }
func (err *ConnectionHandlerError) Network() bool   { return isNetwork(err.Err) }
func (err *ConnectionHandlerError) Timeout() bool   { return isTimeout(err.Err) }
func (err *ConnectionHandlerError) Temporary() bool { return isTemporary(err.Err) }
func (err *ConnectionHandlerError) Canceled() bool  { return isCanceled(err.Err) }
func (err *ConnectionHandlerError) Expired() bool   { return isExpired(err.Err) }
func (err *ConnectionHandlerError) EOF() bool {
	if err.Err == io.EOF {
		return true
	}
	ok, _ := regexp.MatchString("(?i)eof", err.Err.Error())
	return ok
}
func (err *ConnectionHandlerError) Error() string {
	if err == nil {
		return "<nil>"
	}

	fields := make(map[string]interface{})

	if err.HandlerPtr != "" {
		fields["handler_ptr"] = err.HandlerPtr
	}
	if err.Net != "" {
		fields["network"] = err.Net
	}
	if err.LAddr != "" {
		fields["local_addr"] = err.LAddr
	}
	if err.RAddr != "" {
		fields["remote_addr"] = err.RAddr
	}

	return fmt.Sprintf("transport.ConnectionHandlerError<%s>: %s", fields, err.Err)
}

// 监听服务错误
type ListenerHandlerError struct {
	Err        error
	Key        ListenerKey
	HandlerPtr string
	Net        string
	Addr       string
}

func (err *ListenerHandlerError) Unwrap() error   { return err.Err }
func (err *ListenerHandlerError) Network() bool   { return isNetwork(err.Err) }
func (err *ListenerHandlerError) Timeout() bool   { return isTimeout(err.Err) }
func (err *ListenerHandlerError) Temporary() bool { return isTemporary(err.Err) }
func (err *ListenerHandlerError) Canceled() bool  { return isCanceled(err.Err) }
func (err *ListenerHandlerError) Expired() bool   { return isExpired(err.Err) }
func (err *ListenerHandlerError) Error() string {
	if err == nil {
		return "<nil>"
	}

	fields := make(map[string]interface{})

	if err.HandlerPtr != "" {
		fields["handler_ptr"] = err.HandlerPtr
	}
	if err.Net != "" {
		fields["network"] = err.Net
	}
	if err.Addr != "" {
		fields["local_addr"] = err.Addr
	}

	return fmt.Sprintf("transport.ListenerHandlerError<%s>: %s", fields, err.Err)
}

// 连接池错误
type PoolError struct {
	Err  error
	Op   string
	Pool string
}

func (err *PoolError) Unwrap() error   { return err.Err }
func (err *PoolError) Network() bool   { return isNetwork(err.Err) }
func (err *PoolError) Timeout() bool   { return isTimeout(err.Err) }
func (err *PoolError) Temporary() bool { return isTemporary(err.Err) }
func (err *PoolError) Error() string {
	if err == nil {
		return "<nil>"
	}

	fields := make(map[string]interface{})

	if err.Pool != "" {
		fields["pool"] = err.Pool
	}

	return fmt.Sprintf("transport.PoolError<%s> %s failed: %s", fields, err.Op, err.Err)
}

// 不支持的协议错误
type UnsupportedProtocolError string

func (err UnsupportedProtocolError) Network() bool   { return false }
func (err UnsupportedProtocolError) Timeout() bool   { return false }
func (err UnsupportedProtocolError) Temporary() bool { return false }
func (err UnsupportedProtocolError) Error() string {
	return "transport.UnsupportedProtocolError: " + string(err)
}
