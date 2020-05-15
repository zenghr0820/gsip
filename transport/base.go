package transport

import (
	"time"
)

const (
	MTU uint = 1500

	// IPv4 max size - IPv4 Header size - UDP Header size
	BufferSize      uint16 = 65535 - 20 - 8
	NetErrRetryTime        = 5 * time.Second
	SockTTL                = time.Hour
)
