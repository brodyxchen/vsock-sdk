package statistics

import (
	"github.com/brodyxchen/vsock-sdk/statistics/metrics"
	"time"
)

var (
	serverCloseChan chan struct{}

	ServerReg metrics.Registry

	EnableServer = false
)

func InitServer() {
	serverCloseChan = make(chan struct{})

	ServerReg = metrics.NewRegistry()
}

func RunServer() {
	if EnableServer {
		metrics.LogRoutine("Server", ServerReg, 10*time.Second, serverCloseChan)
	}
}

func CloseServer() {
	close(serverCloseChan)
	ServerReg.UnregisterAll()
}
