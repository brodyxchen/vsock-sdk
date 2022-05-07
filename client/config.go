package client

import (
	"github.com/brodyxchen/vsock/constant"
	"time"
)

type Config struct {
	Timeout         time.Duration
	PoolIdleTimeout time.Duration
	PoolMaxCapacity int
	WriteBufferSize int
	ReadBufferSize  int
}

func (cfg *Config) GetTimeout() time.Duration {
	if cfg.Timeout > 0 {
		return cfg.Timeout
	}
	return constant.ClientTimeout
}
func (cfg *Config) GetPoolIdleTimeout() time.Duration {
	if cfg.PoolIdleTimeout > 0 {
		return cfg.PoolIdleTimeout
	}
	return constant.MaxConnPoolIdleTimeout
}
func (cfg *Config) GetPoolMaxCapacity() int {
	if cfg.PoolMaxCapacity > 0 {
		return cfg.PoolMaxCapacity
	}
	return constant.MaxConnPoolCapacity
}
func (cfg *Config) GetWriteBufferSize() int {
	if cfg.WriteBufferSize > 0 {
		return cfg.WriteBufferSize
	}
	return constant.MaxWriteBufferSize
}
func (cfg *Config) GetReadBufferSize() int {
	if cfg.ReadBufferSize > 0 {
		return cfg.ReadBufferSize
	}
	return constant.MaxReadBufferSize
}
