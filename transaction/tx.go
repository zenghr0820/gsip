package transaction

import (
	"fmt"
	"strings"

	"github.com/discoviking/fsm"
	"github.com/zenghr0820/gsip/sip"
	"github.com/zenghr0820/gsip/transport"
)

type TxKey string

func (key TxKey) String() string {
	return string(key)
}

// 定义通用的事务
type Tx interface {
	// 初始化事务：INVITE 事务以及非 INVITE 事务
	Init() error
	// 事务唯一 key
	Key() TxKey
	// 返回创建该事物的请求
	Origin() sip.Request
	// 接收从传输层传递的信息
	Receive(msg sip.Message) error
	String() string
	// 返回传输层实例
	Transport() transport.Layer
	// 返回 session
	Session() sip.Session
	// 关闭事务
	Close()
	// 返回接收异常通道
	Errors() <-chan error
	// 返回是否完成通道
	Done() <-chan bool
}

// 实现通用的事务层
type commonTx struct {
	// 事务唯一 key
	key TxKey
	// 事务状态机：控制事务改变状态
	fsm *fsm.FSM
	// 请求来源
	origin sip.Request
	// 传输层
	tpl transport.Layer
	// session
	session sip.Session
	// 最后接收到的响应
	lastResp sip.Response
	// 接收异常通道
	errs chan error
	// 最后接收到的异常
	lastErr error
	// 判断该事物关闭是否完成
	done chan bool
}

func (tx *commonTx) String() string {
	if tx == nil {
		return "<nil>"
	}

	return fmt.Sprintf("<%s>", tx.key)
}

func (tx *commonTx) Origin() sip.Request {
	return tx.origin
}

func (tx *commonTx) Key() TxKey {
	return tx.key
}

func (tx *commonTx) Transport() transport.Layer {
	return tx.tpl
}

func (tx *commonTx) Session() sip.Session {
	return tx.session
}

func (tx *commonTx) Errors() <-chan error {
	return tx.errs
}

func (tx *commonTx) Done() <-chan bool {
	return tx.done
}

// MakeServerTxKey creates server commonTx key for matching retransmitting requests - RFC 3261 17.2.3.
// 创建 服务端事务 秘钥
func MakeServerTxKey(msg sip.Message) (TxKey, error) {
	var sep = "__"

	firstViaHop, ok := msg.ViaHop()
	if !ok {
		return "", fmt.Errorf("'Via' header not found or empty in message '%s'", msg.Short())
	}

	seq, ok := msg.CSeq()
	if !ok {
		return "", fmt.Errorf("'CSeq' header not found in message '%s'", msg.Short())
	}
	method := seq.MethodName
	if method == sip.ACK || method == sip.CANCEL {
		method = sip.INVITE
	}

	var isRFC3261 bool
	branch, ok := firstViaHop.Params.Get("branch")
	if ok && branch.String() != "" &&
		strings.HasPrefix(branch.String(), sip.RFC3261BranchMagicCookie) &&
		strings.TrimPrefix(branch.String(), sip.RFC3261BranchMagicCookie) != "" {

		isRFC3261 = true
	} else {
		isRFC3261 = false
	}

	// RFC 3261 compliant
	if isRFC3261 {
		var port sip.Port

		if firstViaHop.Port == nil {
			port = sip.DefaultPort(firstViaHop.Transport)
		} else {
			port = *firstViaHop.Port
		}

		return TxKey(strings.Join([]string{
			branch.String(),  // branch
			firstViaHop.Host, // sent-by Host
			fmt.Sprint(port), // sent-by Port
			string(method),   // request Method
		}, sep)), nil
	}
	// RFC 2543 compliant
	from, ok := msg.From()
	if !ok {
		return "", fmt.Errorf("'From' header not found in message '%s'", msg.Short())
	}
	fromTag, ok := from.Params.Get("tag")
	if !ok {
		return "", fmt.Errorf("'tag' param not found in 'From' header of message '%s'", msg.Short())
	}
	callId, ok := msg.CallID()
	if !ok {
		return "", fmt.Errorf("'Call-ID' header not found in message '%s'", msg.Short())
	}

	return TxKey(strings.Join([]string{
		// TODO: how to match core.Response in Send method to server tx? currently disabled
		// msg.Recipient().String(), // request-uri
		fromTag.String(),      // from tag
		callId.String(),       // Call-ID
		string(method),        // cseq method
		fmt.Sprint(seq.SeqNo), // cseq num
		firstViaHop.String(),  // top Via
	}, sep)), nil
}

// MakeClientTxKey creates client commonTx key for matching responses - RFC 3261 17.1.3.
// 创建用于匹配响应的客户端commonTx密钥
func MakeClientTxKey(msg sip.Message) (TxKey, error) {
	var sep = "__"

	cseq, ok := msg.CSeq()
	if !ok {
		return "", fmt.Errorf("'CSeq' header not found in message '%s'", msg.Short())
	}
	method := cseq.MethodName
	if method == sip.ACK {
		method = sip.INVITE
	}

	firstViaHop, ok := msg.ViaHop()
	if !ok {
		return "", fmt.Errorf("'Via' header not found or empty in message '%s'", msg.Short())
	}

	branch, ok := firstViaHop.Params.Get("branch")
	if !ok || len(branch.String()) == 0 ||
		!strings.HasPrefix(branch.String(), sip.RFC3261BranchMagicCookie) ||
		len(strings.TrimPrefix(branch.String(), sip.RFC3261BranchMagicCookie)) == 0 {
		return "", fmt.Errorf("'branch' not found or empty in 'Via' header of message '%s'", msg.Short())
	}

	return TxKey(strings.Join([]string{
		branch.String(),
		string(method),
	}, sep)), nil
}
