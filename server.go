package vsock

import (
	"github.com/brodyxchen/vsock/models"
	"github.com/brodyxchen/vsock/server"
	"time"
)

func NewServer(addr models.Addr) *server.Server {
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
