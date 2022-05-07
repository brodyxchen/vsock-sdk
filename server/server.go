package server

import (
	"context"
	"github.com/brodyxchen/vsock/log"
	"github.com/brodyxchen/vsock/models"
	"github.com/mdlayher/vsock"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type handleFunc func([]byte) ([]byte, error)

type Server struct {
	Addr models.Addr

	handlers map[string]handleFunc
	mutex    sync.RWMutex

	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	DisableKeepAlives int32 // accessed atomically.

	connIndex int64 // atomic visit
}

func (srv *Server) getConnIndex() int64 {
	value := atomic.AddInt64(&srv.connIndex, 1)
	return value
}

func (srv *Server) Init() {
	srv.handlers = make(map[string]handleFunc, 0)
	srv.mutex = sync.RWMutex{}
}

func (srv *Server) HandleFunc(path string, handleFn handleFunc) {
	srv.mutex.Lock()
	defer srv.mutex.Unlock()
	srv.handlers[path] = handleFn
}

func (srv *Server) getHandler(path string) handleFunc {
	srv.mutex.RLock()
	defer srv.mutex.RUnlock()
	handler, ok := srv.handlers[path]
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
	index := srv.getConnIndex()
	c := &Conn{
		Name:   "srv-" + strconv.FormatInt(index, 10),
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
