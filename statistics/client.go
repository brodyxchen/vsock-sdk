package statistics

import (
	"github.com/brodyxchen/vsock-sdk/statistics/metrics"
	"time"
)

var (
	clientCloseChan chan struct{}

	ClientReg metrics.Registry

	EnableClient = false
)

func InitClient() {
	clientCloseChan = make(chan struct{})

	ClientReg = metrics.NewRegistry()
}

func RunClient() {
	if EnableClient {
		metrics.LogRoutine("Client", ClientReg, 10*time.Second, clientCloseChan)
	}
}

func CloseClient() {
	close(clientCloseChan)
	ClientReg.UnregisterAll()
}
