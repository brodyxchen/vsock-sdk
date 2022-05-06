package constant

import "time"

const (
	MaxReadBufferSize  = 4 << 10
	MaxWriteBufferSize = 4 << 10

	MaxConnPoolCountPerKey = 2048

	MaxConnPoolIdleTimeout = time.Minute

	MaxReadBytes = 4 << 10
)
