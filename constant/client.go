package constant

import "time"

const (
	MaxReadBufferSize  = 4 << 10
	MaxWriteBufferSize = 4 << 10

	MaxConnPoolCountPerKey = 10

	MaxConnPoolIdleTimeout = time.Minute

	MaxReadBytes = 4 << 10
)
