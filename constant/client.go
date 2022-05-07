package constant

import "time"

const (
	ClientTimeout = time.Second * 10

	MaxReadBufferSize  = 4 << 10
	MaxWriteBufferSize = 4 << 10

	MaxConnPoolCapacity = 1024 * 2

	MaxConnPoolIdleTimeout = time.Minute
)
