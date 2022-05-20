package statistics

import (
	"github.com/brodyxchen/vsock/statistics/metrics"
	"time"
)

var (
	clientCloseChan chan struct{}

	ClientReg metrics.Registry
)

func InitClient() {
	clientCloseChan = make(chan struct{})

	ClientReg = metrics.NewRegistry()
}

func RunClient() {
	metrics.LogRoutine("Client", ClientReg, 10*time.Second, clientCloseChan)
}

func CloseClient() {
	close(clientCloseChan)
	ClientReg.UnregisterAll()
}
