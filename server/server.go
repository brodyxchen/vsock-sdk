package server

import (
	"context"
	"github.com/brodyxchen/vsock/log"
	"github.com/brodyxchen/vsock/models"
	"github.com/mdlayher/vsock"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	ActionError = uint16(0)
)

type handleFunc func(uint16, []byte) []byte

type Server struct {
	Addr models.Addr

	handlers map[uint16]handleFunc
	mutex    sync.RWMutex

	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	DisableKeepAlives int32 // accessed atomically.
}

func (srv *Server) Init() {
	srv.handlers = make(map[uint16]handleFunc, 0)
	srv.mutex = sync.RWMutex{}
}

func (srv *Server) HandleAction(action uint16, handleFn handleFunc) {
	if action == ActionError {
		panic("invalid action")
	}

	srv.mutex.Lock()
	defer srv.mutex.Unlock()
	srv.handlers[action] = handleFn
}

func (srv *Server) getHandler(action uint16) handleFunc {
	srv.mutex.RLock()
	defer srv.mutex.RUnlock()
	handler, ok := srv.handlers[action]
	if !ok {
		return nil
	}
	return handler
}

func (srv *Server) ListenAndServe() error {
	var (
		ln  net.Listener
		err error
	)

	switch adr := srv.Addr.(type) {
	case *models.VSockAddr:
		ln, err = vsock.ListenContextID(adr.ContextId, adr.Port, nil)
	case *models.HttpAddr:
		ln, err = net.Listen("tcp", adr.GetAddr())
	}

	if err != nil {
		return err
	}

	return srv.Serve(ln)

}

func (srv *Server) Serve(l net.Listener) error {
	log.Debugf("srv.Serve(%v)...\n", srv.Addr.GetAddr())
	defer l.Close()
	ctx := context.Background()

	var tempDelay time.Duration // how long to sleep on accept failure

	for {
		rw, err := l.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				tempDelay = srv.sleep(tempDelay)
				continue
			}
			return err
		}

		log.Debug("l.accept conn...")
		connCtx := ctx
		tempDelay = 0

		c := srv.newConn(rw)
		go c.serve(connCtx)
	}
}

// Create new connection from rwc.
func (srv *Server) newConn(rwc net.Conn) *Conn {
	c := &Conn{
		server: srv,
		rwc:    rwc,
	}
	return c
}

func (srv *Server) doKeepAlives() bool {
	return atomic.LoadInt32(&srv.DisableKeepAlives) == 0
}

func (srv *Server) idleTimeout() time.Duration {
	if srv.IdleTimeout != 0 {
		return srv.IdleTimeout
	}
	return srv.ReadTimeout
}

func (srv *Server) sleep(tempDelay time.Duration) time.Duration {
	if tempDelay == 0 {
		tempDelay = 5 * time.Millisecond
	} else {
		tempDelay *= 2
	}
	if max := 1 * time.Second; tempDelay > max {
		tempDelay = max
	}
	time.Sleep(tempDelay)
	return tempDelay
}
