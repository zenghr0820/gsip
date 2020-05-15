package transaction

import (
	"fmt"
	"sync"
	"time"

	"github.com/discoviking/fsm"
	"github.com/zenghr0820/gsip/logger"
	"github.com/zenghr0820/gsip/sip"
	"github.com/zenghr0820/gsip/transport"
)

// 定义服务端事务以及实现
func NewServerTx(origin sip.Request, tpl transport.Layer) (ServerTx, error) {
	key, err := MakeServerTxKey(origin)
	if err != nil {
		return nil, err
	}

	tx := new(serverTx)
	tx.key = key
	tx.tpl = tpl
	tx.session = sip.CreateSession()
	// about ~10 retransmits
	tx.ackRequest = make(chan sip.Request, 64)
	tx.cancelRequest = make(chan sip.Request, 64)
	tx.errs = make(chan error, 64)
	tx.done = make(chan bool)

	tx.origin = origin.(sip.Request)

	if viaHop, ok := origin.ViaHop(); ok {
		tx.reliable = tx.tpl.IsReliable(viaHop.Transport)
	}

	return tx, nil
}

type ServerTx interface {
	Tx
	// 返回响应
	SendResponse(res sip.Response) error
	// ACK 请求
	AckRequest() <-chan sip.Request
	// 取消一个请求 RFC 3261 - 9.0.
	CancelRequest() <-chan sip.Request
}

// 实现
type serverTx struct {
	// 通用事务
	commonTx
	// 最后一个 Ack 请求
	lastAck sip.Request
	// 最后一个 Cancel 请求
	lastCancel sip.Request
	// ack 请求传输通道
	ackRequest chan sip.Request
	// cancel 请求传输通道
	cancelRequest chan sip.Request
	// 定时器定义 RFC 3261 - 17.2.1 INVITE 服务端事务
	timerG    *time.Timer
	timeGTime time.Duration
	timerH    *time.Timer
	timerI    *time.Timer
	timeITime time.Duration
	timerJ    *time.Timer
	timer1xx  *time.Timer
	// 判断是否传输协议是否可靠
	reliable bool
	// 锁
	mu sync.RWMutex
	// 确保关闭方法只执行一次
	closeOnce sync.Once
}

func (tx *serverTx) Init() error {
	logger.Debugf("[serverTx] -> received SIP request:\n%s", tx.origin)

	tx.initFSM()

	tx.mu.Lock()

	if tx.reliable {
		tx.timeGTime = 0
		tx.timeITime = 0
	} else {
		// 必须设置定时器 G=T1 秒
		tx.timeGTime = TimeG
		// 不可靠传输协议，那么就要设定一个定时器 I=T4 秒
		tx.timeITime = TimeI
	}

	tx.mu.Unlock()

	// RFC 3261 - 17.2.1
	// INVITE必须在 200ms 生成 100(Trying)应答
	// TODO 需要改写 zhr
	if tx.Origin().IsInvite() {
		logger.Debugf("[serverTx] -> set timer1xx to %v", Time1xx)

		tx.mu.Lock()
		tx.timer1xx = time.AfterFunc(Time1xx, func() {
			logger.Debug("[serverTx] -> timer1xx fired")

			if err := tx.SendResponse(
				// 创建该请求对应的响应 100
				tx.Origin().CreateResponseReason(sip.StatusTrying, "Trying"),
			); err != nil {
				logger.Errorf("[serverTx] -> send '100 Trying' response failed: %s", err)
			}
		})
		tx.mu.Unlock()
	}

	return nil
}

// 接收 sip 信息
func (tx *serverTx) Receive(msg sip.Message) error {
	req, ok := msg.(sip.Request)
	if !ok {
		return &sip.UnexpectedMessageError{
			Err: fmt.Errorf("[serverTx] ->  %s recevied unexpected %s", tx, msg),
			Msg: req.String(),
		}
	}

	logger.Infof("[serverTx] -> received SIP request:\n%s", req.Method())

	tx.mu.Lock()
	if tx.timer1xx != nil {
		tx.timer1xx.Stop()
		tx.timer1xx = nil
	}
	tx.mu.Unlock()

	var input = fsm.NO_INPUT
	switch {
	case req.Method() == tx.Origin().Method():
		// 重发请求
		input = serverInputRequest
	case req.IsAck(): // ACK for non-2xx response
		// ack
		input = serverInputAck
		tx.mu.Lock()
		tx.lastAck = req
		tx.mu.Unlock()
	case req.IsCancel():
		// 取消请求
		input = serverInputCancel
		tx.mu.Lock()
		tx.lastCancel = req
		tx.mu.Unlock()
	default:
		return &sip.UnexpectedMessageError{
			Err: fmt.Errorf("[serverTx] -> invalid %s correlated to %s", msg, tx),
			Msg: req.String(),
		}
	}

	return tx.fsm.Spin(input)
}

// 发送响应
func (tx *serverTx) SendResponse(res sip.Response) error {
	if res.IsCancel() { // 是否是取消响应
		return tx.tpl.Send(res)
	}

	tx.mu.Lock()
	tx.lastResp = res

	if tx.timer1xx != nil {
		tx.timer1xx.Stop()
		tx.timer1xx = nil
	}
	tx.mu.Unlock()

	var input fsm.Input
	switch {
	case res.IsProvisional(): // 是否临时应答
		input = serverInputUser1xx
	case res.IsSuccess(): // 最终应答
		input = serverInputUser2xx
	default:
		input = serverInputUser300Plus
	}

	return tx.fsm.Spin(input)
}

func (tx *serverTx) AckRequest() <-chan sip.Request {
	return tx.ackRequest
}

func (tx *serverTx) CancelRequest() <-chan sip.Request {
	return tx.cancelRequest
}

func (tx *serverTx) Close() {
	select {
	case <-tx.done:
		return
	default:
	}

	tx.delete()
}

// 状态机设置
// Choose the right FSM init function depending on request method.
func (tx *serverTx) initFSM() {
	if tx.Origin().IsInvite() {
		tx.initInviteFSM()
	} else {
		tx.initNonInviteFSM()
	}
}

func (tx *serverTx) initInviteFSM() {
	// Define States
	logger.Debug("[serverTx] -> initialising INVITE transaction FSM")

	// Proceeding
	serverStateDefProceeding := fsm.State{
		Index: serverStateProceeding,
		Outcomes: map[fsm.Input]fsm.Outcome{
			serverInputRequest: {serverStateProceeding, tx.actionRespond},
			// 收到 cancel 请求，返回 200，不改变原请求状态，传递给 TU 判断是否终止请求
			serverInputCancel:       {serverStateProceeding, tx.actionCancel},
			serverInputUser1xx:      {serverStateProceeding, tx.actionRespond},
			serverInputUser2xx:      {serverStateTerminated, tx.actionRespondDelete},
			serverInputUser300Plus:  {serverStateCompleted, tx.actionRespondComplete},
			serverInputTransportErr: {serverStateTerminated, tx.actionTransErr},
		},
	}

	// Completed
	serverStateDefCompleted := fsm.State{
		Index: serverStateCompleted,
		Outcomes: map[fsm.Input]fsm.Outcome{
			serverInputRequest: {serverStateCompleted, tx.actionRespond},
			serverInputAck:     {serverStateConfirmed, tx.actionConfirm},
			// 收到 cancel 请求，返回 481，不改变原请求状态
			serverInputCancel:       {serverStateCompleted, tx.actionCancelNotExist},
			serverInputUser1xx:      {serverStateCompleted, fsm.NO_ACTION},
			serverInputUser2xx:      {serverStateCompleted, fsm.NO_ACTION},
			serverInputUser300Plus:  {serverStateCompleted, fsm.NO_ACTION},
			serverInputTimerG:       {serverStateCompleted, tx.actionRespondComplete},
			serverInputTimerH:       {serverStateTerminated, tx.actionDelete},
			serverInputTransportErr: {serverStateTerminated, tx.actionTransErr},
		},
	}

	// Confirmed
	serverStateDefConfirmed := fsm.State{
		Index: serverStateConfirmed,
		Outcomes: map[fsm.Input]fsm.Outcome{
			serverInputRequest: {serverStateConfirmed, fsm.NO_ACTION},
			serverInputAck:     {serverStateTerminated, fsm.NO_ACTION},
			// 收到 cancel 请求，返回 481，不改变原请求状态
			serverInputCancel:      {serverStateConfirmed, tx.actionCancelNotExist},
			serverInputUser1xx:     {serverStateConfirmed, fsm.NO_ACTION},
			serverInputUser2xx:     {serverStateConfirmed, fsm.NO_ACTION},
			serverInputUser300Plus: {serverStateConfirmed, fsm.NO_ACTION},
			serverInputTimerI:      {serverStateTerminated, tx.actionDelete},
			serverInputTimerG:      {serverStateConfirmed, fsm.NO_ACTION},
			serverInputTimerH:      {serverStateConfirmed, fsm.NO_ACTION},
		},
	}

	// Terminated
	serverStateDefTerminated := fsm.State{
		Index: serverStateTerminated,
		Outcomes: map[fsm.Input]fsm.Outcome{
			serverInputRequest:     {serverStateTerminated, fsm.NO_ACTION},
			serverInputAck:         {serverStateTerminated, fsm.NO_ACTION},
			serverInputCancel:      {serverStateTerminated, fsm.NO_ACTION},
			serverInputUser1xx:     {serverStateTerminated, fsm.NO_ACTION},
			serverInputUser2xx:     {serverStateTerminated, fsm.NO_ACTION},
			serverInputUser300Plus: {serverStateTerminated, fsm.NO_ACTION},
			serverInputDelete:      {serverStateTerminated, tx.actionDelete},
		},
	}

	// Define FSM
	fsm_, err := fsm.Define(
		serverStateDefProceeding,
		serverStateDefCompleted,
		serverStateDefConfirmed,
		serverStateDefTerminated,
	)
	if err != nil {
		logger.Errorf("[serverTx] -> define INVITE transaction FSM failed: %s", err)

		return
	}

	tx.fsm = fsm_
}

func (tx *serverTx) initNonInviteFSM() {
	// Define States
	logger.Debug("[serverTx] -> initialising non-INVITE transaction FSM")

	// Trying
	serverStateDefTrying := fsm.State{
		Index: serverStateTrying,
		Outcomes: map[fsm.Input]fsm.Outcome{
			serverInputRequest:     {serverStateTrying, fsm.NO_ACTION},
			serverInputCancel:      {serverStateConfirmed, fsm.NO_ACTION},
			serverInputUser1xx:     {serverStateProceeding, tx.actionRespond},
			serverInputUser2xx:     {serverStateCompleted, tx.actionFinal},
			serverInputUser300Plus: {serverStateCompleted, tx.actionFinal},
		},
	}

	// Proceeding
	serverStateDefProceeding := fsm.State{
		Index: serverStateProceeding,
		Outcomes: map[fsm.Input]fsm.Outcome{
			serverInputRequest:      {serverStateProceeding, tx.actionRespond},
			serverInputCancel:       {serverStateConfirmed, fsm.NO_ACTION},
			serverInputUser1xx:      {serverStateProceeding, tx.actionRespond},
			serverInputUser2xx:      {serverStateCompleted, tx.actionFinal},
			serverInputUser300Plus:  {serverStateCompleted, tx.actionFinal},
			serverInputTransportErr: {serverStateTerminated, tx.actionTransErr},
		},
	}

	// Completed
	serverStateDefCompleted := fsm.State{
		Index: serverStateCompleted,
		Outcomes: map[fsm.Input]fsm.Outcome{
			serverInputRequest:      {serverStateCompleted, tx.actionRespond},
			serverInputCancel:       {serverStateConfirmed, fsm.NO_ACTION},
			serverInputUser1xx:      {serverStateCompleted, fsm.NO_ACTION},
			serverInputUser2xx:      {serverStateCompleted, fsm.NO_ACTION},
			serverInputUser300Plus:  {serverStateCompleted, fsm.NO_ACTION},
			serverInputTimerJ:       {serverStateTerminated, tx.actionDelete},
			serverInputTransportErr: {serverStateTerminated, tx.actionTransErr},
		},
	}

	// Terminated
	serverStateDefTerminated := fsm.State{
		Index: serverStateTerminated,
		Outcomes: map[fsm.Input]fsm.Outcome{
			serverInputRequest:     {serverStateTerminated, fsm.NO_ACTION},
			serverInputCancel:      {serverStateConfirmed, fsm.NO_ACTION},
			serverInputUser1xx:     {serverStateTerminated, fsm.NO_ACTION},
			serverInputUser2xx:     {serverStateTerminated, fsm.NO_ACTION},
			serverInputUser300Plus: {serverStateTerminated, fsm.NO_ACTION},
			serverInputTimerJ:      {serverStateTerminated, fsm.NO_ACTION},
			serverInputDelete:      {serverStateTerminated, tx.actionDelete},
		},
	}

	// Define FSM
	fsm_, err := fsm.Define(
		serverStateDefTrying,
		serverStateDefProceeding,
		serverStateDefCompleted,
		serverStateDefTerminated,
	)
	if err != nil {
		logger.Errorf("[serverTx] -> define non-INVITE FSM failed: %s", err)

		return
	}

	tx.fsm = fsm_
}

func (tx *serverTx) transportErr() {
	// todo bloody patch
	defer func() { recover() }()

	tx.mu.RLock()
	res := tx.lastResp
	err := tx.lastErr
	tx.mu.RUnlock()

	err = &TxTransportError{
		fmt.Errorf("[serverTx] -> transaction failed to send %s: %w", res, err),
		tx.Key(),
		fmt.Sprintf("%p", tx),
	}

	select {
	case <-tx.done:
	case tx.errs <- err:
	}
}

func (tx *serverTx) timeoutErr() {
	// todo bloody patch
	defer func() { recover() }()

	err := &TxTimeoutError{
		fmt.Errorf("[serverTx] -> transaction timed out"),
		tx.Key(),
		fmt.Sprintf("%p", tx),
	}

	select {
	case <-tx.done:
	case tx.errs <- err:
	}
}

func (tx *serverTx) delete() {
	select {
	case <-tx.done:
		return
	default:
	}
	// todo bloody patch
	defer func() { recover() }()

	tx.mu.Lock()
	if tx.timerI != nil {
		tx.timerI.Stop()
		tx.timerI = nil
	}
	if tx.timerG != nil {
		tx.timerG.Stop()
		tx.timerG = nil
	}
	if tx.timerH != nil {
		tx.timerH.Stop()
		tx.timerH = nil
	}
	if tx.timerJ != nil {
		tx.timerJ.Stop()
		tx.timerJ = nil
	}
	if tx.timer1xx != nil {
		tx.timer1xx.Stop()
		tx.timer1xx = nil
	}
	tx.mu.Unlock()

	time.Sleep(time.Microsecond)

	tx.closeOnce.Do(func() {
		tx.mu.Lock()

		close(tx.done)
		close(tx.ackRequest)
		close(tx.cancelRequest)
		close(tx.errs)

		tx.mu.Unlock()
	})
}

/**
定义状态机改变执行的动作
*/

// 发送响应
func (tx *serverTx) actionRespond() fsm.Input {
	tx.mu.RLock()
	lastResp := tx.lastResp
	tx.mu.RUnlock()

	if lastResp == nil {
		return fsm.NO_INPUT
	}

	logger.Info("[serverTx] -> actionRespond")

	lastErr := tx.tpl.Send(lastResp)

	tx.mu.Lock()
	tx.lastErr = lastErr
	tx.mu.Unlock()

	if lastErr != nil {
		return serverInputTransportErr
	}

	return fsm.NO_INPUT
}

// 转到 Completed 状态
func (tx *serverTx) actionRespondComplete() fsm.Input {
	tx.mu.RLock()
	lastResp := tx.lastResp
	tx.mu.RUnlock()

	if lastResp == nil {
		return fsm.NO_INPUT
	}

	logger.Debug("actionRespond_complete")

	lastErr := tx.tpl.Send(lastResp)

	tx.mu.Lock()
	tx.lastErr = lastErr
	tx.mu.Unlock()

	if lastErr != nil {
		return serverInputTransportErr
	}

	if !tx.reliable {
		tx.mu.Lock()
		if tx.timerG == nil {
			logger.Debugf("timerG set to %v", tx.timeGTime)

			tx.timerG = time.AfterFunc(tx.timeGTime, func() {
				logger.Debug("timerG fired")

				if err := tx.fsm.Spin(serverInputTimerG); err != nil {
					logger.Errorf("spin FSM to serverInputTimerG failed: %s", err)
				}
			})
		} else {
			tx.timeGTime *= 2
			if tx.timeGTime > T2 {
				tx.timeGTime = T2
			}

			logger.Debugf("timerG reset to %v", tx.timeGTime)

			tx.timerG.Reset(tx.timeGTime)
		}
		tx.mu.Unlock()
	}

	tx.mu.Lock()
	if tx.timerH == nil {
		logger.Debugf("timerH set to %v", TimeH)

		tx.timerH = time.AfterFunc(TimeH, func() {
			logger.Debug("timerH fired")

			if err := tx.fsm.Spin(serverInputTimerH); err != nil {
				logger.Errorf("spin FSM to serverInputTimerH failed: %s", err)
			}
		})
	}
	tx.mu.Unlock()

	return fsm.NO_INPUT
}

// Send final response
// 发送最终应答
func (tx *serverTx) actionFinal() fsm.Input {
	tx.mu.RLock()
	lastResp := tx.lastResp
	tx.mu.RUnlock()

	if lastResp == nil {
		return fsm.NO_INPUT
	}

	logger.Info("[serverTx] -> actionFinal")

	lastErr := tx.tpl.Send(tx.lastResp)

	tx.mu.Lock()
	tx.lastErr = lastErr
	tx.mu.Unlock()

	if lastErr != nil {
		return serverInputTransportErr
	}

	tx.mu.Lock()

	logger.Debugf("[serverTx] -> timerJ set to %v", TimeJ)

	tx.timerJ = time.AfterFunc(TimeJ, func() {
		logger.Debug("[serverTx] -> timerJ fired")

		if err := tx.fsm.Spin(serverInputTimerJ); err != nil {
			logger.Errorf("[serverTx] -> spin FSM to serverInputTimerJ failed: %s", err)
		}
	})

	tx.mu.Unlock()

	return fsm.NO_INPUT
}

// Inform user of transport error
// 触发传输层错误
func (tx *serverTx) actionTransErr() fsm.Input {
	logger.Debug("[serverTx] -> actionTransErr")

	tx.transportErr()

	return serverInputDelete
}

// Inform user of timeout error
// 触发超时动作
func (tx *serverTx) actionTimeout() fsm.Input {
	logger.Debug("[serverTx] -> actTimeout")

	tx.timeoutErr()

	return serverInputDelete
}

// Just delete the transaction.
func (tx *serverTx) actionDelete() fsm.Input {
	logger.Debug("[serverTx] -> actionDelete")

	tx.delete()

	return fsm.NO_INPUT
}

// Send response and delete the transaction.
func (tx *serverTx) actionRespondDelete() fsm.Input {
	logger.Debug("[serverTx] -> actionRespond_delete")

	tx.delete()

	tx.mu.RLock()
	lastErr := tx.tpl.Send(tx.lastResp)
	tx.mu.RUnlock()

	tx.mu.Lock()
	tx.lastErr = lastErr
	tx.mu.Unlock()

	if lastErr != nil {
		return serverInputTransportErr
	}

	return fsm.NO_INPUT
}

// 切换 Confirm 状态
func (tx *serverTx) actionConfirm() fsm.Input {
	logger.Debug("[serverTx] -> actionConfirm")

	// todo bloody patch
	defer func() { recover() }()

	tx.mu.Lock()

	if tx.timerG != nil {
		tx.timerG.Stop()
		tx.timerG = nil
	}

	if tx.timerH != nil {
		tx.timerH.Stop()
		tx.timerH = nil
	}

	logger.Debugf("[serverTx] -> timerI set to %v", TimeI)

	tx.timerI = time.AfterFunc(TimeI, func() {
		logger.Debug("[serverTx] -> timerI fired")

		if err := tx.fsm.Spin(serverInputTimerI); err != nil {
			logger.Errorf("[serverTx] -> spin FSM to serverInputTimerI failed: %s", err)
		}
	})

	tx.mu.Unlock()

	tx.mu.RLock()
	ack := tx.lastAck
	tx.mu.RUnlock()

	if ack != nil {
		select {
		case <-tx.done:
		case tx.ackRequest <- ack:
		}
	}

	return fsm.NO_INPUT
}

// 取消请求 Action
func (tx *serverTx) actionCancel() fsm.Input {
	logger.Debug("[serverTx] -> actionCancel")

	// todo bloody patch
	defer func() { recover() }()

	tx.mu.RLock()
	cancel := tx.lastCancel
	tx.mu.RUnlock()

	if cancel != nil {
		// cancel 请求返回200响应
		err := tx.tpl.Send(cancel.CreateResponse(sip.StatusOK))
		if err != nil {
			logger.Error("[serverTx] -> Cancel Response 200 Error: ", err)
		}

		select {
		case <-tx.done:
		case tx.cancelRequest <- cancel:
		}
	}

	return fsm.NO_INPUT
}

// 取消的请求不存在
func (tx *serverTx) actionCancelNotExist() fsm.Input {
	logger.Debug("[serverTx] -> actionCancelNotExist")

	// todo bloody patch
	defer func() { recover() }()

	tx.mu.RLock()
	cancel := tx.lastCancel
	tx.mu.RUnlock()

	if cancel != nil {
		// e.g: RFC 3261 - 9.2
		// 找不到对应的事务 返回 481 应答
		err := tx.tpl.Send(cancel.CreateResponse(sip.StatusCallTransactionDoesNotExist))
		logger.Error("[serverTx] -> Cancel Response 481 Error: ", err)
	}

	return fsm.NO_INPUT
}
