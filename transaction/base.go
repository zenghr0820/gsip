package transaction

import (
	"time"

	"github.com/discoviking/fsm"
)

// 基础常量、方法等定义
const (
	T1      = 500 * time.Millisecond
	T2      = 4 * time.Second
	T4      = 5 * time.Second
	TimeA   = T1
	TimeB   = 64 * T1
	TimeD   = 32 * time.Second
	TimeE   = T1
	TimeF   = 64 * T1
	TimeG   = T1
	TimeH   = 64 * T1
	TimeI   = T4
	TimeJ   = 64 * T1
	TimeK   = T4
	Time1xx = 200 * time.Millisecond
)

// 客户端事务状态机状态 FSM States
const (
	clientStateCalling = iota
	clientStateProceeding
	clientStateCompleted
	clientStateTerminated
)

// FSM Inputs
// 状态机改变状态事件
const (
	clientInput1xx fsm.Input = iota
	clientInput2xx
	clientInput300Plus
	clientInputTimerA
	clientInputTimerB
	clientInputTimerD
	clientInputTransportErr
	clientInputDelete
)

// 服务端事务状态机状态 FSM States
const (
	serverStateTrying = iota
	serverStateProceeding
	serverStateCompleted
	serverStateConfirmed
	serverStateTerminated
)

// FSM Inputs
// 状态机改变状态事件
const (
	serverInputRequest fsm.Input = iota
	serverInputAck
	serverInputCancel
	serverInputUser1xx
	serverInputUser2xx
	serverInputUser300Plus
	serverInputTimerG
	serverInputTimerH
	serverInputTimerI
	serverInputTimerJ
	serverInputTransportErr
	serverInputDelete
)
