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

type ClientTx interface {
	Tx
	Responses() <-chan sip.Response
}

type clientTx struct {
	commonTx
	responses chan sip.Response
	timeATime time.Duration // Current duration of timer A.
	timerA    *time.Timer
	timerB    *time.Timer
	timeDTime time.Duration // Current duration of timer D.
	timerD    *time.Timer
	reliable  bool

	mu        sync.RWMutex
	closeOnce sync.Once
}

func NewClientTx(origin sip.Request, tpl transport.Layer) (ClientTx, error) {
	key, err := MakeClientTxKey(origin)
	if err != nil {
		return nil, err
	}

	tx := new(clientTx)
	tx.key = key
	tx.tpl = tpl
	tx.session = sip.CreateSession()
	tx.origin = origin
	// buffer chan - about ~10 retransmit responses
	tx.responses = make(chan sip.Response, 64)
	tx.errs = make(chan error, 64)
	tx.done = make(chan bool)

	if viaHop, ok := origin.ViaHop(); ok {
		tx.reliable = tx.tpl.IsReliable(viaHop.Transport)
	}

	return tx, nil
}

func (tx *clientTx) Init() error {
	tx.initFSM()

	logger.Infof("[clientTx] -> sending SIP request:\n%s", tx.origin.Short())

	if err := tx.tpl.Send(tx.Origin()); err != nil {
		tx.mu.Lock()
		tx.lastErr = err
		tx.mu.Unlock()

		if err := tx.fsm.Spin(clientInputTransportErr); err != nil {
			logger.Errorf("[clientTx] -> spin FSM to clientInputTransportErr failed: %s", err)
		}

		return err
	}

	if tx.reliable {
		tx.mu.Lock()
		tx.timeDTime = 0
		tx.mu.Unlock()
	} else {
		// RFC 3261 - 17.1.1.2.
		// If an unreliable transport is being used, the client transaction MUST start timer A with a value of T1.
		// If a reliable transport is being used, the client transaction SHOULD NOT
		// start timer A (Timer A controls request retransmissions).
		// Timer A - retransmission
		// 对于非可靠传输(比如 UDP)，客户端事务每隔 T1 重发请求，每次重发后间隔时间加倍
		logger.Debugf("[clientTx] -> timerA on E set to %v", T1)

		tx.mu.Lock()
		// Timer D is set to 32 seconds for unreliable transports
		if tx.Origin().IsInvite() {
			tx.timeATime = TimeA
			tx.timeDTime = TimeD
		} else {
			tx.timeATime = TimeE
			tx.timeDTime = TimeK
		}

		tx.timerA = time.AfterFunc(tx.timeATime, func() {
			logger.Debug("[clientTx] -> timerA on E fired")

			if err := tx.fsm.Spin(clientInputTimerA); err != nil {
				logger.Errorf("[clientTx] -> spin FSM to clientInputTimerA on E failed: %s", err)
			}
		})

		tx.mu.Unlock()
	}

	// Timer B on F - timeout
	logger.Debugf("[clientTx] -> timerB On F set to %v", TimeB)

	tx.mu.Lock()
	tx.timerB = time.AfterFunc(TimeB, func() {
		logger.Debug("[clientTx] -> timerB On F fired")

		if err := tx.fsm.Spin(clientInputTimerB); err != nil {
			logger.Errorf("[clientTx] -> spin FSM to clientInputTimerB On F failed: %s", err)
		}
	})
	tx.mu.Unlock()

	tx.mu.RLock()
	err := tx.lastErr
	tx.mu.RUnlock()

	return err
}

func (tx *clientTx) Receive(msg sip.Message) error {
	res, ok := msg.(sip.Response)
	if !ok {
		return &sip.UnexpectedMessageError{
			Err: fmt.Errorf("%s recevied unexpected %s", tx, msg.Short()),
			Msg: msg.String(),
		}
	}

	tx.mu.Lock()
	tx.lastResp = res
	tx.mu.Unlock()

	var input fsm.Input
	switch {
	case res.IsProvisional():
		input = clientInput1xx
	case res.IsSuccess():
		input = clientInput2xx
	default:
		input = clientInput300Plus
	}

	return tx.fsm.Spin(input)
}

func (tx *clientTx) Responses() <-chan sip.Response {
	return tx.responses
}

// 关闭、释放资源
func (tx *clientTx) Close() {
	select {
	case <-tx.done:
		return
	default:
	}

	tx.delete()
}

// 返回 ack 请求
func (tx clientTx) ack() {
	tx.mu.RLock()
	lastResp := tx.lastResp
	tx.mu.RUnlock()

	// 创建对应的 ack 请求
	ackRequest := lastResp.CreateAck()

	// Send the ACK.
	err := tx.tpl.Send(ackRequest)
	if err != nil {
		logger.Warnf("[clientTx] -> send ACK request failed: %s", err)

		tx.mu.Lock()
		tx.lastErr = err
		tx.mu.Unlock()

		if err := tx.fsm.Spin(clientInputTransportErr); err != nil {
			logger.Errorf("[clientTx] -> spin FSM to clientInputTransportErr failed: %s", err)
		}
	}
}

// Initialises the correct kind of FSM based on request method.
func (tx *clientTx) initFSM() {
	if tx.Origin().IsInvite() {
		tx.initInviteFSM()
	} else {
		tx.initNonInviteFSM()
	}
}

func (tx *clientTx) initInviteFSM() {
	logger.Debug("[clientTx] -> initialising INVITE transaction FSM")

	// Define States
	// Calling
	clientStateDefCalling := fsm.State{
		Index: clientStateCalling,
		Outcomes: map[fsm.Input]fsm.Outcome{
			clientInput1xx:          {clientStateProceeding, tx.actionPassUp},
			clientInput2xx:          {clientStateTerminated, tx.actionPassUpDelete},
			clientInput300Plus:      {clientStateCompleted, tx.actionInviteFinal},
			clientInputTimerA:       {clientStateCalling, tx.actionInviteResend},
			clientInputTimerB:       {clientStateTerminated, tx.actionTimeout},
			clientInputTransportErr: {clientStateTerminated, tx.actionTransErr},
		},
	}

	// Proceeding
	clientStateDefProceeding := fsm.State{
		Index: clientStateProceeding,
		Outcomes: map[fsm.Input]fsm.Outcome{
			clientInput1xx:     {clientStateProceeding, tx.actionPassUp},
			clientInput2xx:     {clientStateTerminated, tx.actionPassUpDelete},
			clientInput300Plus: {clientStateCompleted, tx.actionInviteFinal},
			clientInputTimerA:  {clientStateProceeding, fsm.NO_ACTION},
			clientInputTimerB:  {clientStateProceeding, fsm.NO_ACTION},
		},
	}

	// Completed
	clientStateDefCompleted := fsm.State{
		Index: clientStateCompleted,
		Outcomes: map[fsm.Input]fsm.Outcome{
			clientInput1xx:          {clientStateCompleted, fsm.NO_ACTION},
			clientInput2xx:          {clientStateCompleted, fsm.NO_ACTION},
			clientInput300Plus:      {clientStateCompleted, tx.actionAck},
			clientInputTransportErr: {clientStateTerminated, tx.actionTransErr},
			clientInputTimerA:       {clientStateCompleted, fsm.NO_ACTION},
			clientInputTimerB:       {clientStateCompleted, fsm.NO_ACTION},
			clientInputTimerD:       {clientStateTerminated, tx.actionDelete},
		},
	}

	// Terminated
	clientStateDefTerminated := fsm.State{
		Index: clientStateTerminated,
		Outcomes: map[fsm.Input]fsm.Outcome{
			clientInput1xx:     {clientStateTerminated, fsm.NO_ACTION},
			clientInput2xx:     {clientStateTerminated, fsm.NO_ACTION},
			clientInput300Plus: {clientStateTerminated, fsm.NO_ACTION},
			clientInputTimerA:  {clientStateTerminated, fsm.NO_ACTION},
			clientInputTimerB:  {clientStateTerminated, fsm.NO_ACTION},
			clientInputTimerD:  {clientStateTerminated, fsm.NO_ACTION},
			clientInputDelete:  {clientStateTerminated, tx.actionDelete},
		},
	}

	fsm_, err := fsm.Define(
		clientStateDefCalling,
		clientStateDefProceeding,
		clientStateDefCompleted,
		clientStateDefTerminated,
	)

	if err != nil {
		logger.Errorf("[clientTx] -> define INVITE transaction FSM failed: %s", err)

		return
	}

	tx.fsm = fsm_
}

func (tx *clientTx) initNonInviteFSM() {
	logger.Debug("[clientTx] -> initialising non-INVITE transaction FSM")

	// Define States
	// "Trying"
	clientStateDefCalling := fsm.State{
		Index: clientStateCalling,
		Outcomes: map[fsm.Input]fsm.Outcome{
			clientInput1xx:          {clientStateProceeding, tx.actionPassUp},
			clientInput2xx:          {clientStateCompleted, tx.actionNonInviteFinal},
			clientInput300Plus:      {clientStateCompleted, tx.actionNonInviteFinal},
			clientInputTimerA:       {clientStateCalling, tx.actionNonInviteResend},
			clientInputTimerB:       {clientStateTerminated, tx.actionTimeout},
			clientInputTransportErr: {clientStateTerminated, tx.actionTransErr},
		},
	}

	// Proceeding
	clientStateDefProceeding := fsm.State{
		Index: clientStateProceeding,
		Outcomes: map[fsm.Input]fsm.Outcome{
			clientInput1xx:          {clientStateProceeding, tx.actionPassUp},
			clientInput2xx:          {clientStateCompleted, tx.actionNonInviteFinal},
			clientInput300Plus:      {clientStateCompleted, tx.actionNonInviteFinal},
			clientInputTimerA:       {clientStateProceeding, tx.actionNonInviteResend},
			clientInputTimerB:       {clientStateTerminated, tx.actionTimeout},
			clientInputTransportErr: {clientStateTerminated, tx.actionTransErr},
		},
	}

	// Completed
	clientStateDefCompleted := fsm.State{
		Index: clientStateCompleted,
		Outcomes: map[fsm.Input]fsm.Outcome{
			clientInput1xx:     {clientStateCompleted, fsm.NO_ACTION},
			clientInput2xx:     {clientStateCompleted, fsm.NO_ACTION},
			clientInput300Plus: {clientStateCompleted, fsm.NO_ACTION},
			clientInputTimerA:  {clientStateCompleted, fsm.NO_ACTION},
			clientInputTimerB:  {clientStateCompleted, fsm.NO_ACTION},
			clientInputTimerD:  {clientStateTerminated, tx.actionDelete},
		},
	}

	// Terminated
	clientStateDefTerminated := fsm.State{
		Index: clientStateTerminated,
		Outcomes: map[fsm.Input]fsm.Outcome{
			clientInput1xx:     {clientStateTerminated, fsm.NO_ACTION},
			clientInput2xx:     {clientStateTerminated, fsm.NO_ACTION},
			clientInput300Plus: {clientStateTerminated, fsm.NO_ACTION},
			clientInputTimerA:  {clientStateTerminated, fsm.NO_ACTION},
			clientInputTimerB:  {clientStateTerminated, fsm.NO_ACTION},
			clientInputTimerD:  {clientStateTerminated, fsm.NO_ACTION},
			clientInputDelete:  {clientStateTerminated, tx.actionDelete},
		},
	}

	fsm_, err := fsm.Define(
		clientStateDefCalling,
		clientStateDefProceeding,
		clientStateDefCompleted,
		clientStateDefTerminated,
	)

	if err != nil {
		logger.Errorf("[clientTx] -> define non-INVITE transaction FSM failed: %s", err)

		return
	}

	tx.fsm = fsm_
}

// 重发请求
func (tx *clientTx) resend() {
	logger.Debug("[clientTx] -> resend origin request")

	err := tx.tpl.Send(tx.Origin())

	tx.mu.Lock()
	tx.lastErr = err
	tx.mu.Unlock()

	if err != nil {
		if err := tx.fsm.Spin(clientInputTransportErr); err != nil {
			logger.Errorf("[clientTx] -> spin FSM to clientInputTransportErr failed: %s", err)
		}
	}
}

// 往上层传递
func (tx *clientTx) passUp() {
	tx.mu.RLock()
	lastResp := tx.lastResp
	tx.mu.RUnlock()

	if lastResp != nil {
		select {
		case <-tx.done:
		case tx.responses <- lastResp:
		}
	}
}

// 传输层异常
func (tx *clientTx) transportErr() {
	// todo bloody patch
	defer func() { recover() }()

	tx.mu.RLock()
	res := tx.lastResp
	err := tx.lastErr
	tx.mu.RUnlock()

	err = &TxTransportError{
		fmt.Errorf("transaction failed to send %s: %w", res.Short(), err),
		tx.Key(),
		fmt.Sprintf("%p", tx),
	}

	select {
	case <-tx.done:
	case tx.errs <- err:
	}
}

// 超时异常
func (tx *clientTx) timeoutErr() {
	// todo bloody patch
	defer func() { recover() }()

	err := &TxTimeoutError{
		fmt.Errorf("[clientTx] -> transaction timed out"),
		tx.Key(),
		fmt.Sprintf("%p", tx),
	}

	select {
	case <-tx.done:
	case tx.errs <- err:
	}
}

func (tx *clientTx) delete() {
	select {
	case <-tx.done:
		return
	default:
	}
	// todo bloody patch
	defer func() { recover() }()

	tx.mu.Lock()
	if tx.timerA != nil {
		tx.timerA.Stop()
		tx.timerA = nil
	}
	if tx.timerB != nil {
		tx.timerB.Stop()
		tx.timerB = nil
	}
	if tx.timerD != nil {
		tx.timerD.Stop()
		tx.timerD = nil
	}
	tx.mu.Unlock()

	time.Sleep(time.Microsecond)

	tx.closeOnce.Do(func() {
		tx.mu.Lock()

		close(tx.done)
		close(tx.responses)
		close(tx.errs)

		tx.mu.Unlock()
	})
}

// Define actions
// 重发 invite 请求
func (tx *clientTx) actionInviteResend() fsm.Input {
	logger.Debug("[clientTx] -> actionInviteResend")

	tx.mu.Lock()

	tx.timeATime *= 2
	tx.timerA.Reset(tx.timeATime)

	tx.mu.Unlock()

	tx.resend()

	return fsm.NO_INPUT
}

// 重发非 invite请求
func (tx *clientTx) actionNonInviteResend() fsm.Input {
	logger.Debug("[clientTx] -> actionNonInviteResend")

	tx.mu.Lock()

	tx.timeATime *= 2
	// For non-INVITE, cap timer A at T2 seconds.
	if tx.timeATime > T2 {
		tx.timeATime = T2
	}
	tx.timerA.Reset(tx.timeATime)

	tx.mu.Unlock()

	tx.resend()

	return fsm.NO_INPUT
}

func (tx *clientTx) actionPassUp() fsm.Input {
	logger.Debug("[clientTx] -> actionPassUp")

	tx.passUp()

	tx.mu.Lock()

	if tx.timerA != nil {
		tx.timerA.Stop()
		tx.timerA = nil
	}

	tx.mu.Unlock()

	return fsm.NO_INPUT
}

func (tx *clientTx) actionInviteFinal() fsm.Input {
	logger.Debug("[clientTx] -> actionInviteFinal")

	tx.passUp()
	tx.ack()

	tx.mu.Lock()

	if tx.timerA != nil {
		tx.timerA.Stop()
		tx.timerA = nil
	}
	if tx.timerD != nil {
		tx.timerD.Stop()
		tx.timerD = nil
	}

	logger.Debugf("[clientTx] -> timerD set to %v", tx.timeDTime)

	tx.timerD = time.AfterFunc(tx.timeDTime, func() {
		logger.Debug("[clientTx] -> timerD fired")

		if err := tx.fsm.Spin(clientInputTimerD); err != nil {
			logger.Errorf("[clientTx] -> spin FSM to clientInputTimerD failed: %s", err)
		}
	})

	tx.mu.Unlock()

	return fsm.NO_INPUT
}

func (tx *clientTx) actionNonInviteFinal() fsm.Input {
	logger.Debug("[clientTx] -> actionNonInviteFinal")

	tx.passUp()

	tx.mu.Lock()

	if tx.timerA != nil {
		tx.timerA.Stop()
		tx.timerA = nil
	}
	if tx.timerD != nil {
		tx.timerD.Stop()
		tx.timerD = nil
	}

	logger.Debugf("[clientTx] -> timerD set to %v", tx.timeDTime)

	tx.timerD = time.AfterFunc(tx.timeDTime, func() {
		logger.Debug("[clientTx] -> timerD fired")

		if err := tx.fsm.Spin(clientInputTimerD); err != nil {
			logger.Errorf("[clientTx] -> spin FSM to clientInputTimerD failed: %s", err)
		}
	})

	tx.mu.Unlock()

	return fsm.NO_INPUT
}

func (tx *clientTx) actionAck() fsm.Input {
	logger.Debug("[clientTx] -> actionAck")

	tx.ack()

	return fsm.NO_INPUT
}

func (tx *clientTx) actionTransErr() fsm.Input {
	logger.Debug("[clientTx] -> actionTransErr")

	tx.transportErr()

	tx.mu.Lock()

	if tx.timerA != nil {
		tx.timerA.Stop()
		tx.timerA = nil
	}

	tx.mu.Unlock()

	return clientInputDelete
}

// 定时器 B 触发，通知 TU 超时
func (tx *clientTx) actionTimeout() fsm.Input {
	logger.Debug("[clientTx] -> actionTimeout")

	tx.timeoutErr()

	tx.mu.Lock()

	if tx.timerA != nil {
		tx.timerA.Stop()
		tx.timerA = nil
	}

	tx.mu.Unlock()

	return clientInputDelete
}

func (tx *clientTx) actionPassUpDelete() fsm.Input {
	logger.Debug("[clientTx] -> actionPassUpDelete")

	tx.passUp()

	tx.mu.Lock()

	if tx.timerA != nil {
		tx.timerA = nil
	}

	tx.mu.Unlock()

	return clientInputDelete
}

func (tx *clientTx) actionDelete() fsm.Input {
	logger.Debug("[clientTx] -> actionDelete")

	tx.delete()

	return fsm.NO_INPUT
}
