package transaction

import (
	"fmt"
	"sync"
	"time"

	"github.com/zenghr0820/gsip/logger"
	"github.com/zenghr0820/gsip/sip"
	"github.com/zenghr0820/gsip/transport"
)

// 创建实例化事务层
func CreateLayer(tpl transport.Layer) Layer {
	txl := &layer{
		tpl:          tpl,
		transactions: createTransactionPool(),

		requests:   make(chan sip.ServerTransaction),
		ackRequest: make(chan sip.Request),
		responses:  make(chan sip.Response),

		errs:     make(chan error),
		done:     make(chan struct{}),
		canceled: make(chan struct{}),
	}

	go txl.listenMessages()

	return txl
}

// 事务层定义与实现
type Layer interface {
	Send(message sip.Message) (sip.Transaction, error)
	// 传输层实例
	Transport() transport.Layer
	// Requests returns channel with new incoming server transactions.
	Requests() <-chan sip.ServerTransaction
	// ACKs on 2xx
	AckRequest() <-chan sip.Request
	// Responses returns channel with not matched responses.
	Responses() <-chan sip.Response
	Errors() <-chan error
	// 关闭释放资源
	Close()
	// 等待关闭动作完成
	Done() <-chan struct{}
	String() string
}

type layer struct {
	tpl          transport.Layer
	requests     chan sip.ServerTransaction
	ackRequest   chan sip.Request
	responses    chan sip.Response
	transactions *transactionPool

	errs     chan error
	done     chan struct{}
	canceled chan struct{}

	txWg       sync.WaitGroup
	cancelOnce sync.Once
}

// 返回 请求传输通道
func (txl *layer) Requests() <-chan sip.ServerTransaction {
	return txl.requests
}

// 返回 Ack 请求传输通道
func (txl *layer) AckRequest() <-chan sip.Request {
	return txl.ackRequest
}

// 返回响应传输通道
func (txl *layer) Responses() <-chan sip.Response {
	return txl.responses
}

// 返回异常现象
func (txl *layer) Errors() <-chan error {
	return txl.errs
}

// 返回传输层
func (txl *layer) Transport() transport.Layer {
	return txl.tpl
}

// 发送请求或者响应
func (txl *layer) Send(message sip.Message) (sip.Transaction, error) {
	logger.Debug("[txl_layer] -> Start parsing information type ")
	switch msg := message.(type) {
	case sip.Request:
		return txl.sendRequest(msg)
	case sip.Response:
		return txl.sendResponse(msg)
	default:
		logger.Error("[txl_layer] -> message type error")
	}

	return nil, fmt.Errorf("[txl_layer] -> message type mismatch")
}

func (txl *layer) String() string {
	if txl == nil {
		return "<nil>"
	}

	return fmt.Sprintf("transaction.Layer<%s>", "txl")
}

// 释放资源
func (txl *layer) Close() {
	select {
	case <-txl.canceled:
		return
	default:
	}

	txl.cancelOnce.Do(func() {
		close(txl.canceled)

		logger.Debug("transaction layer canceled")
	})
}

// 确认关闭操作是否完成
func (txl *layer) Done() <-chan struct{} {
	return txl.done
}

// 发送请求
func (txl *layer) sendRequest(req sip.Request) (sip.ClientTransaction, error) {
	select {
	case <-txl.canceled:
		return nil, fmt.Errorf("[txl_layer] -> transaction layer is canceled")
	default:
	}

	// RFC 3621 - 17.0 对于 ACK 来说，是不存在客户事务的
	if req.IsAck() {
		err := txl.tpl.Send(req)
		return nil, err
	}

	tx, err := NewClientTx(req, txl.tpl)
	if err != nil {
		return nil, err
	}

	logger.Debug("[txl_layer] -> client transaction created")

	txl.transactions.put(tx.Key(), tx)

	err = tx.Init()
	if err != nil {
		return nil, err
	}

	txl.txWg.Add(1)
	go txl.serveTransaction(tx)

	return tx, nil
}

// 发送响应
func (txl *layer) sendResponse(res sip.Response) (sip.ServerTransaction, error) {
	select {
	case <-txl.canceled:
		return nil, fmt.Errorf("[txl_layer] -> transaction layer is canceled")
	default:
	}

	tx, err := txl.getServerTx(res)
	if err != nil {
		return nil, err
	}

	err = tx.SendResponse(res)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

// 监听传输层传递的数据
func (txl *layer) listenMessages() {
	defer func() {
		txl.txWg.Add(1)
		go func() {
			time.Sleep(time.Millisecond)
			txl.txWg.Done()
		}()

		txl.txWg.Wait()

		close(txl.requests)
		close(txl.responses)
		close(txl.errs)
		close(txl.done)
	}()

	logger.Info("[txl_layer] -> start listen messages")
	defer logger.Info("[txl_layer] -> stop listen messages")

	for {
		select {
		case <-txl.canceled:
			return
		case msg, ok := <-txl.tpl.GetMessage():
			if !ok {
				return
			}

			go txl.handleMessage(msg)
		}
	}
}

// 监听该事务取消，释放资源
func (txl *layer) serveTransaction(tx Tx) {
	defer func() {
		txl.transactions.drop(tx.Key())

		logger.Debug("[txl_layer] -> transaction deleted")

		txl.txWg.Done()
	}()

	logger.Debug("[txl_layer] -> start serve transaction")
	defer logger.Debug("[txl_layer] -> stop serve transaction")

	for {
		select {
		case <-txl.canceled:
			tx.Close()
			return
		case <-tx.Done():
			return

		}
	}
}

// 处理数据
func (txl *layer) handleMessage(msg sip.Message) {
	select {
	case <-txl.canceled:
		return
	default:
	}

	logger.Debug("[txl_layer] -> handling SIP message")

	switch msg := msg.(type) {
	case sip.Request:
		txl.handleRequest(msg)
	case sip.Response:
		txl.handleResponse(msg)
	default:
		logger.Error("[txl_layer] -> unsupported message, skip it")
		// todo pass up error?
	}
}

// 处理请求
func (txl *layer) handleRequest(req sip.Request) {
	select {
	case <-txl.canceled:
		return
	default:
	}

	// try to match to existent tx: request retransmission, or ACKs on non-2xx, or CANCEL
	tx, err := txl.getServerTx(req)
	if err == nil {
		if err := tx.Receive(req); err != nil {
			logger.Error(err)
		}

		return
	}
	// ACK on 2xx
	if req.IsAck() {
		select {
		case <-txl.canceled:
		case txl.ackRequest <- req:
		}
		return
	}
	if req.IsCancel() {
		// e.g: RFC 3261 - 9.2
		// 找不到对应的事务 返回 481 应答
		_ = txl.tpl.Send(req.CreateResponse(sip.StatusCallTransactionDoesNotExist))
		return
	}

	// 创建新的服务端事务
	tx, err = NewServerTx(req, txl.tpl)
	if err != nil {
		logger.Error(err)
		return
	}

	logger.Debug("[txl_layer] -> new server transaction created")
	// put tx to store, to match retransmitting requests later
	// 将新的事务 tx 存储在事务池中
	txl.transactions.put(tx.Key(), tx)
	// 初始化该事务
	if err := tx.Init(); err != nil {
		logger.Error(err)

		return
	}

	select {
	case <-txl.canceled:
		return
	default:
	}

	txl.txWg.Add(1)
	// 监听该事务取消，释放资源
	go txl.serveTransaction(tx)

	// pass up request
	// 往上层传递
	logger.Info("[txl_layer] -> passing up SIP request...")

	select {
	case <-txl.canceled:
	case txl.requests <- tx:
		logger.Info("[txl_layer] -> SIP request passed up [TU]")
	}
}

// 处理响应
func (txl *layer) handleResponse(res sip.Response) {
	select {
	case <-txl.canceled:
		return
	default:
	}

	// 获取事务池中对应的客户端事务
	tx, err := txl.getClientTx(res)
	if err != nil {

		logger.Info("[txl_layer] -> passing up non-matched SIP response")
		// RFC 3261 - 17.1.1.2.
		// Not matched responses should be passed directly to the UA
		// 不匹配的响应应直接传递给UA
		select {
		case <-txl.canceled:
		case txl.responses <- res:
			logger.Info("[txl_layer] -> non-matched SIP response passed up")
		}
		return

	}

	if err := tx.Receive(res); err != nil {
		logger.Error(err)
	}

}

// RFC 17.1.3.
func (txl *layer) getClientTx(msg sip.Message) (ClientTx, error) {
	logger.Info("[txl_layer] -> searching client transaction")

	key, err := MakeClientTxKey(msg)
	if err != nil {
		return nil, fmt.Errorf("[getClientTx] -> %s failed to match message '%s' to client transaction: %s", txl, msg.Short(), err)
	}

	tx, ok := txl.transactions.get(key)
	if !ok {
		return nil, fmt.Errorf(
			"[getClientTx] -> %s failed to match message '%s' to client transaction: transaction with key '%s' not found",
			txl,
			msg.Short(),
			key,
		)
	}

	switch tx := tx.(type) {
	case ClientTx:
		logger.Info("[txl_layer] -> client transaction found")

		return tx, nil
	default:
		return nil, fmt.Errorf(
			"[getClientTx] -> %s failed to match message '%s' to client transaction: found %s is not a client transaction",
			txl,
			msg.Short(),
			tx,
		)
	}
}

// RFC 17.2.3.
func (txl *layer) getServerTx(msg sip.Message) (ServerTx, error) {
	logger.Info("[txl_layer] -> searching server transaction")

	key, err := MakeServerTxKey(msg)
	if err != nil {
		return nil, fmt.Errorf("[getServerTx] -> %s failed to match message '%s' to server transaction: %s", txl, msg.Short(), err)
	}

	tx, ok := txl.transactions.get(key)
	if !ok {
		return nil, fmt.Errorf(
			"[getServerTx] -> %s failed to match message '%s' to server transaction: transaction with key '%s' not found",
			txl,
			msg.Short(),
			key,
		)
	}

	switch tx := tx.(type) {
	case ServerTx:
		logger.Info("[txl_layer] -> server transaction found")

		return tx, nil
	default:
		return nil, fmt.Errorf(
			"[getServerTx] -> %s failed to match message '%s' to server transaction: found %s is not server transaction",
			txl,
			msg.Short(),
			tx,
		)
	}
}
