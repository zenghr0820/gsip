package transaction

// 定义事务层常见异常
import (
	"fmt"
)

type TxError interface {
	error
	Key() TxKey
	Terminated() bool
	Timeout() bool
	Transport() bool
}

// 事务终止异常
type TxTerminatedError struct {
	Err   error
	TxKey TxKey
	TxPtr string
}

func (err *TxTerminatedError) Unwrap() error    { return err.Err }
func (err *TxTerminatedError) Terminated() bool { return true }
func (err *TxTerminatedError) Timeout() bool    { return false }
func (err *TxTerminatedError) Transport() bool  { return false }
func (err *TxTerminatedError) Key() TxKey       { return err.TxKey }
func (err *TxTerminatedError) Error() string {
	if err == nil {
		return "<nil>"
	}

	fields := map[string]interface{}{
		"transaction_key": "???",
		"transaction_ptr": "???",
	}

	if err.TxKey != "" {
		fields["transaction_key"] = err.TxKey
	}
	if err.TxPtr != "" {
		fields["transaction_ptr"] = err.TxPtr
	}

	return fmt.Sprintf("transaction.TxTerminatedError<%s>: %s", fields, err.Err)
}

// 事务超时异常
type TxTimeoutError struct {
	Err   error
	TxKey TxKey
	TxPtr string
}

func (err *TxTimeoutError) Unwrap() error    { return err.Err }
func (err *TxTimeoutError) Terminated() bool { return false }
func (err *TxTimeoutError) Timeout() bool    { return true }
func (err *TxTimeoutError) Transport() bool  { return false }
func (err *TxTimeoutError) Key() TxKey       { return err.TxKey }
func (err *TxTimeoutError) Error() string {
	if err == nil {
		return "<nil>"
	}

	fields := map[string]interface{}{
		"transaction_key": "???",
		"transaction_ptr": "???",
	}

	if err.TxKey != "" {
		fields["transaction_key"] = err.TxKey
	}
	if err.TxPtr != "" {
		fields["transaction_ptr"] = err.TxPtr
	}

	return fmt.Sprintf("transaction.TxTimeoutError<%s>: %s", fields, err.Err)
}

// 传输层异常
type TxTransportError struct {
	Err   error
	TxKey TxKey
	TxPtr string
}

func (err *TxTransportError) Unwrap() error    { return err.Err }
func (err *TxTransportError) Terminated() bool { return false }
func (err *TxTransportError) Timeout() bool    { return false }
func (err *TxTransportError) Transport() bool  { return true }
func (err *TxTransportError) Key() TxKey       { return err.TxKey }
func (err *TxTransportError) Error() string {
	if err == nil {
		return "<nil>"
	}

	fields := map[string]interface{}{
		"transaction_key": "???",
		"transaction_ptr": "???",
	}

	if err.TxKey != "" {
		fields["transaction_key"] = err.TxKey
	}
	if err.TxPtr != "" {
		fields["transaction_ptr"] = err.TxPtr
	}

	return fmt.Sprintf("transaction.TxTransportError<%s>: %s", fields, err.Err)
}
