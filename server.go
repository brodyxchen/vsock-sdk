package vsock

import (
	"github.com/brodyxchen/vsock/models"
	"github.com/brodyxchen/vsock/server"
	"time"
)

func NewServer(addr models.Addr) *server.Server {
	srv := &server.Server{
		Addr:              addr,
		ReadTimeout:       time.Second,
		WriteTimeout:      time.Second,
		IdleTimeout:       time.Minute,
		DisableKeepAlives: 0,
	}
	srv.Init()
	return srv
}
