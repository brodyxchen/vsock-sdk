package statistics

import (
	"cryptobroker/vsock-sdk/statistics/metrics"
	"time"
)

var (
	serverCloseChan chan struct{}

	ServerReg metrics.Registry
)

func InitServer() {
	serverCloseChan = make(chan struct{})

	ServerReg = metrics.NewRegistry()
}

func RunServer() {
	metrics.LogRoutine("Server", ServerReg, 10*time.Second, serverCloseChan)
}

func CloseServer() {
	close(serverCloseChan)
	ServerReg.UnregisterAll()
}
