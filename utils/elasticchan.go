// Forked from github.com/StefanKopieczek/gossip by @StefanKopieczek
package utils

import "github.com/zenghr0820/gsip/logger"

// The buffer size of the primitive input and output chans.
// 初始化 输入、输出通道大小
const cElasticChanSize = 3

// A dynamic channel that does not block on send, but has an unlimited buffer capacity.
// ElasticChan uses a dynamic slice to buffer signals received on the input channel until
// the output channel is ready to process them.
// 一种动态信道，发送时不阻塞，但具有无限的缓冲容量
// ElasticChan使用动态切片缓冲在输入通道上接收到的信号, 直到输出通道已准备好处理它们。
type ElasticChan struct {
	In      chan interface{} // 输入通道
	Out     chan interface{} // 输出通道
	buffer  []interface{}    // 动态切片
	stopped bool             // 是否停止
	done    chan struct{}    // 是否完成取消
}

// Initialise the Elastic channel
// 初始化弹性通道
func (c *ElasticChan) Init() {
	c.In = make(chan interface{}, cElasticChanSize)
	c.Out = make(chan interface{}, cElasticChanSize)
	c.buffer = make([]interface{}, 0)
	c.done = make(chan struct{})
}

// start the management goroutine.
// 启动管理 goroutine
func (c *ElasticChan) Run() {
	go c.manage()
}

// stop the management goroutine.
// 停止 goroutine
func (c *ElasticChan) Stop() {
	select {
	case <-c.done:
		return
	default:
	}

	close(c.In)
	<-c.done
}

// Poll for input from one end of the channel and add it to the buffer.
// Also poll sending buffered signals out over the output chan.
// 轮询通道一端的输入并将其添加到缓冲区
// 也可以通过输出通道轮询发送缓冲信号
func (c *ElasticChan) manage() {
	defer close(c.done)

loop:
	for {
		if len(c.buffer) > 0 {
			// The buffer has something in it, so try to send as well as
			// receive.
			// (Receive first in order to minimize blocked Send() calls).
			select {
			case in, ok := <-c.In:
				if !ok {
					break loop
				}
				logger.Debugf("[ElasticChan] -> %p gets '%v'", c, in)
				c.buffer = append(c.buffer, in)
			case c.Out <- c.buffer[0]:
				logger.Debugf("[ElasticChan] -> %p sends '%v'", c, c.buffer[0])
				c.buffer = c.buffer[1:]
			}
		} else {
			// The buffer is empty, so there's nothing to send.
			// Just wait to receive.
			// 缓冲区是空的，所以没有要发送的内容
			// 就等着接收吧
			in, ok := <-c.In
			if !ok {
				break loop
			}
			logger.Debugf("[ElasticChan] -> %p gets '%v'", c, in)
			c.buffer = append(c.buffer, in)
		}
	}

	c.dispose()
}

func (c *ElasticChan) dispose() {
	for len(c.buffer) > 0 {
		select {
		case c.Out <- c.buffer[0]:
			c.buffer = c.buffer[1:]
		default:
		}
	}
}
