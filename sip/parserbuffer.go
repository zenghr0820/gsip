// Forked from github.com/StefanKopieczek/gossip by @StefanKopieczek
package sip

import (
	"bufio"
	"bytes"
	"io"

	"github.com/zenghr0820/gsip/logger"
)

// parserBuffer is a specialized buffer for use in the parser.
// parser buffer是用于解析器的专用缓冲区
// It is written to via the non-blocking Write.
// 它是通过非阻塞写入
// It exposes various blocking read methods, which wait until the requested
// data is available, and then return it.
// 它公开了各种阻塞读取方法，这些方法等待请求的数据可用，然后返回
type parserBuffer struct {
	io.Writer
	buffer bytes.Buffer

	// Wraps parserBuffer.pipeReader
	reader *bufio.Reader

	// Don't access this directly except when closing.
	pipeReader *io.PipeReader
}

// Create a new parserBuffer object (see struct comment for object details).
// Note that resources owned by the parserBuffer may not be able to be GCed
// until the Dispose() method is called.
func newParserBuffer() *parserBuffer {
	var pb parserBuffer
	pb.pipeReader, pb.Writer = io.Pipe()
	pb.reader = bufio.NewReader(pb.pipeReader)

	return &pb
}

// Block until the buffer contains at least one CRLF-terminated line.
// 直到缓冲区至少包含一个以CRLF结尾的行
// Return the line, excluding the terminal CRLF, and delete it from the buffer.
// 返回该行，不包括终端CRLF，并将其从缓冲区中删除
// Returns an error if the parserBuffer has been stopped.
// 如果parserBuffer已停止，则返回错误
func (pb *parserBuffer) NextLine() (response string, err error) {
	var buffer bytes.Buffer
	var data string
	var b byte

	// There has to be a better way!
	for {
		data, err = pb.reader.ReadString('\r')
		if err != nil {
			return
		}

		buffer.WriteString(data)

		b, err = pb.reader.ReadByte()
		if err != nil {
			return
		}

		buffer.WriteByte(b)
		if b == '\n' {
			response = buffer.String()
			response = response[:len(response)-2]

			logger.Debugf("return line '%s'", response)

			return
		}
	}
}

// Block until the buffer contains at least n characters.
// 直到缓冲区至少包含n个字符
// Return precisely those n characters, then delete them from the buffer.
// 精确返回这n个字符，然后从缓冲区中删除它们
func (pb *parserBuffer) NextChunk(n int) (response string, err error) {
	var data = make([]byte, n)

	var read int
	for total := 0; total < n; {
		read, err = pb.reader.Read(data[total:])
		total += read
		if err != nil {
			return
		}
	}

	response = string(data)

	logger.Debugf("return content:\n%s", response)

	return
}

// Stop the parser buffer.
func (pb *parserBuffer) Stop() {
	if err := pb.pipeReader.Close(); err != nil {
		logger.Errorf("parser pipe reader close failed: %s", err)
	}

	logger.Info("[parser] -> buffer stopped")
}
