package vsock_sdk

import (
	"github.com/brodyxchen/vsock-sdk/models"
	"time"
)

func LaunchCustomExampleServer(port uint32, readTimeout, writeTimeout, idleTimeout time.Duration, keepAlive bool, handleFn func([]byte) ([]byte, error)) {
	addr := &models.HttpAddr{
		IP:   "127.0.0.1",
		Port: port,
	}

	srv := NewServer(addr)
	srv.ReadTimeout = readTimeout
	srv.WriteTimeout = writeTimeout
	srv.IdleTimeout = idleTimeout
	if keepAlive {
		srv.DisableKeepAlives = 0
	} else {
		srv.DisableKeepAlives = 1
	}

	//type handleFunc func([]byte) ([]byte, error)
	srv.HandleFunc("test", func(bytes []byte) ([]byte, error) {
		return handleFn(bytes)
	})

	isRunning := make(chan struct{})
	go func() {
		close(isRunning)
		if err := srv.ListenAndServe(); err != nil {
			panic(err)
		}
	}()
	<-isRunning
}
