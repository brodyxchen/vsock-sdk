package vsock_sdk

import (
	"github.com/brodyxchen/vsock-sdk/models"
	"github.com/brodyxchen/vsock-sdk/server"
	"github.com/brodyxchen/vsock-sdk/statistics"
	"time"
)

func NewServer(addr models.Addr) *server.Server {
	statistics.InitServer()
	statistics.RunServer()

	srv := &server.Server{
		Addr:              addr,
		ReadTimeout:       time.Second * 5,
		WriteTimeout:      time.Second * 10,
		IdleTimeout:       time.Minute,
		DisableKeepAlives: 0,
	}
	srv.Init()

	return srv
}
