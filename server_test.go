package vsock

import (
	"github.com/brodyxchen/vsock/models"
	"math/rand"
	"time"
)

func LaunchCustomExampleServer(port uint32, readTimeout, writeTimeout, idleTimeout time.Duration, keepAlive bool, handleFn func([]byte) []byte) {
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

	srv.HandleAction(uint16(1), func(u uint16, bytes []byte) []byte {
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

func LaunchExampleServer(port uint32) {
	addr := &models.HttpAddr{
		IP:   "127.0.0.1",
		Port: port,
	}

	srv := NewServer(addr)
	srv.ReadTimeout = time.Second * 1
	srv.WriteTimeout = time.Second * 1
	srv.IdleTimeout = time.Second * 20

	srv.HandleAction(uint16(1), func(u uint16, bytes []byte) []byte {
		req := string(bytes)
		return []byte("rsp:" + req)
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

func LaunchExampleSlowServer(port uint32, cost time.Duration) {
	addr := &models.HttpAddr{
		IP:   "127.0.0.1",
		Port: port,
	}

	server := NewServer(addr)
	server.ReadTimeout = time.Second * 2
	server.WriteTimeout = time.Second * 6
	server.IdleTimeout = time.Second * 10

	server.HandleAction(uint16(1), func(u uint16, bytes []byte) []byte {
		req := string(bytes)
		time.Sleep(cost)
		return []byte("rsp:" + req)
	})

	isRunning := make(chan struct{})
	go func() {
		close(isRunning)
		if err := server.ListenAndServe(); err != nil {
			panic(err)
		}
	}()
	<-isRunning
}

func LaunchTestRandCostServer(port uint32, costMin, costMax time.Duration) {
	addr := &models.HttpAddr{
		IP:   "127.0.0.1",
		Port: port,
	}

	server := NewServer(addr)
	server.ReadTimeout = time.Second * 2
	server.WriteTimeout = time.Second * 6
	server.IdleTimeout = time.Second * 10

	server.HandleAction(uint16(1), func(u uint16, bytes []byte) []byte {
		req := string(bytes)
		scope := int64(costMax) - int64(costMin)
		cost := time.Duration(rand.Int63()%scope + int64(costMin))
		time.Sleep(cost)
		return []byte("rsp:" + req)
	})

	isRunning := make(chan struct{})
	go func() {
		close(isRunning)
		if err := server.ListenAndServe(); err != nil {
			panic(err)
		}
	}()
	<-isRunning
}
