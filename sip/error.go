package sip

import "fmt"

type Error interface {
	error
	// Syntax indicates that this is syntax error
	Syntax() bool
}

// 错误的开始行
type InvalidStartLineError string

func (err InvalidStartLineError) Syntax() bool    { return true }
func (err InvalidStartLineError) Malformed() bool { return false }
func (err InvalidStartLineError) Broken() bool    { return true }
func (err InvalidStartLineError) Error() string {
	return "parser.InvalidStartLineError: " + string(err)
}

// 错误的信息格式
type InvalidMessageFormat string

func (err InvalidMessageFormat) Syntax() bool    { return true }
func (err InvalidMessageFormat) Malformed() bool { return true }
func (err InvalidMessageFormat) Broken() bool    { return true }
func (err InvalidMessageFormat) Error() string   { return "parser.InvalidMessageFormat: " + string(err) }

// 写入错误
type WriteError string

func (err WriteError) Syntax() bool  { return false }
func (err WriteError) Error() string { return "parser.WriteError: " + string(err) }

type CancelError interface {
	Canceled() bool
}

// 过期异常
type ExpireError interface {
	Expired() bool
}

// 消息异常
type MessageError interface {
	error
	// Malformed indicates that message is syntactically valid but has invalid headers, or
	// without required headers.
	// 格式错误表示消息在语法上有效，但具有无效的头，或没有必需的头
	Malformed() bool
	// Broken or incomplete message, or not a SIP message
	// 消息已断开或不完整，或不是SIP消息
	Broken() bool
}

// Broken or incomplete messages, or not a SIP message.
// 该异常表示：消息已断开或不完整，或不是SIP消息
type BrokenMessageError struct {
	Err error
	Msg string
}

func (err *BrokenMessageError) Malformed() bool { return false }
func (err *BrokenMessageError) Broken() bool    { return true }
func (err *BrokenMessageError) Error() string {
	if err == nil {
		return "<nil>"
	}

	s := "BrokenMessageError: " + err.Err.Error()
	if err.Msg != "" {
		s += fmt.Sprintf("\nMessage dump:\n%s", err.Msg)
	}

	return s
}

// syntactically valid but logically invalid message
// 该异常表示：无效的消息
type MalformedMessageError struct {
	Err error
	Msg string
}

func (err *MalformedMessageError) Malformed() bool { return true }
func (err *MalformedMessageError) Broken() bool    { return false }
func (err *MalformedMessageError) Error() string {
	if err == nil {
		return "<nil>"
	}

	s := "MalformedMessageError: " + err.Err.Error()
	if err.Msg != "" {
		s += fmt.Sprintf("\nMessage dump:\n%s", err.Msg)
	}

	return s
}

// 该异常表示：不支持的信息
type UnsupportedMessageError struct {
	Err error
	Msg string
}

func (err *UnsupportedMessageError) Malformed() bool { return true }
func (err *UnsupportedMessageError) Broken() bool    { return false }
func (err *UnsupportedMessageError) Error() string {
	if err == nil {
		return "<nil>"
	}

	s := "UnsupportedMessageError: " + err.Err.Error()
	if err.Msg != "" {
		s += fmt.Sprintf("\nMessage dump:\n%s", err.Msg)
	}

	return s
}

// 该异常表示：意外消息错误
type UnexpectedMessageError struct {
	Err error
	Msg string
}

func (err *UnexpectedMessageError) Broken() bool    { return false }
func (err *UnexpectedMessageError) Malformed() bool { return false }
func (err *UnexpectedMessageError) Error() string {
	if err == nil {
		return "<nil>"
	}

	s := "UnexpectedMessageError: " + err.Err.Error()
	if err.Msg != "" {
		s += fmt.Sprintf("\nMessage dump:\n%s", err.Msg)
	}

	return s
}
