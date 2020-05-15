package transport

import (
	"net"
	"strings"
	"sync"
	"time"

	"github.com/zenghr0820/gsip/logger"
)

type ConnectionKey string

func (key ConnectionKey) String() string {
	return string(key)
}

// Wrapper around net.Conn.
type Connection interface {
	net.Conn

	Key() ConnectionKey
	Network() string
	Streamed() bool
	String() string
	ReadFrom(buf []byte) (num int, remoteAddr net.Addr, err error)
	WriteTo(buf []byte, remoteAddr net.Addr) (num int, err error)
}

// Connection implementation.
type connection struct {
	baseConn   net.Conn
	key        ConnectionKey
	localAddr  net.Addr
	remoteAddr net.Addr
	streamed   bool
	mu         sync.RWMutex
}

func CreateConnection(key ConnectionKey, baseConn net.Conn) Connection {
	var stream bool
	switch baseConn.(type) {
	case net.PacketConn:
		stream = false
	default:
		stream = true
	}

	conn := &connection{
		baseConn:   baseConn,
		key:        key,
		localAddr:  baseConn.LocalAddr(),
		remoteAddr: baseConn.RemoteAddr(),
		streamed:   stream,
	}

	return conn
}

func (conn *connection) Key() ConnectionKey {
	return conn.key
}

func (conn *connection) Streamed() bool {
	return conn.streamed
}

func (conn *connection) String() string {
	return "实现我 String()..."
}

func (conn *connection) Network() string {
	return strings.ToUpper(conn.baseConn.LocalAddr().Network())
}

// 基于流式协议网络
func (conn *connection) Read(buf []byte) (int, error) {
	var (
		num int
		err error
	)
	num, err = conn.baseConn.Read(buf)
	if err != nil {
		// TODO error zhr
		return num, err
	}
	logger.Debugf("conn read %d bytes %s <- %s:\n%s", num, conn.LocalAddr(), conn.RemoteAddr(), buf[:num])

	return num, err
}

// 基于网络包协议网络
func (conn *connection) ReadFrom(buf []byte) (num int, remoteAddr net.Addr, err error) {
	num, remoteAddr, err = conn.baseConn.(net.PacketConn).ReadFrom(buf)
	if err != nil {
		// TODO error zhr
		return num, remoteAddr, err
	}
	logger.Debugf("conn ReadFrom %d bytes %s <- %s:\n%s", num, conn.LocalAddr(), remoteAddr, buf[:num])

	return num, remoteAddr, err
}

// 基于流式协议网络
func (conn *connection) Write(buf []byte) (int, error) {
	var (
		num int
		err error
	)

	num, err = conn.baseConn.Write(buf)
	if err != nil {
		return num, err
	}
	logger.Debugf("conn write %d bytes %s -> %s:\n%s", num, conn.LocalAddr(), conn.RemoteAddr(), buf[:num])

	return num, err
}

// 基于网络包协议网络
func (conn *connection) WriteTo(buf []byte, remoteAddr net.Addr) (num int, err error) {
	num, err = conn.baseConn.(net.PacketConn).WriteTo(buf, remoteAddr)
	if err != nil {
		return num, err
	}
	logger.Debugf("conn WriteTo %d bytes %s -> %s:\n%s", num, conn.LocalAddr(), remoteAddr, buf[:num])

	return num, err
}

func (conn *connection) LocalAddr() net.Addr {
	return conn.baseConn.LocalAddr()
}

func (conn *connection) RemoteAddr() net.Addr {
	return conn.baseConn.RemoteAddr()
}

func (conn *connection) Close() error {
	err := conn.baseConn.Close()
	if err != nil {
		return nil
	}
	logger.Info("base connection closed")

	return nil
}

func (conn *connection) SetDeadline(t time.Time) error {
	return conn.baseConn.SetDeadline(t)
}

func (conn *connection) SetReadDeadline(t time.Time) error {
	return conn.baseConn.SetReadDeadline(t)
}

func (conn *connection) SetWriteDeadline(t time.Time) error {
	return conn.baseConn.SetWriteDeadline(t)
}
