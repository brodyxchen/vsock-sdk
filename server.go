package vsock

import (
	"context"
	"errors"
	"fmt"
	"github.com/mdlayher/vsock"
	"net"
	"sync/atomic"
	"time"
)

const (
	ActionError = uint16(0)
)

var (
	errExceedBody    = errors.New("exceed body size")
	errInvalidHeader = errors.New("invalid header")
	errInvalidBody   = errors.New("invalid body")
)

type handleFunc func(uint16, []byte) ([]byte, error)

func NewServer(addr Addr) *Server {
	return &Server{
		Addr:              addr,
		handlers:          make(map[uint16]*Handler, 0),
		ReadTimeout:       0,
		WriteTimeout:      0,
		IdleTimeout:       time.Hour,
		disableKeepAlives: 0,
	}
}

type Handler struct {
	Handle handleFunc
}

type Server struct {
	Addr Addr

	handlers map[uint16]*Handler

	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	IdleTimeout time.Duration

	disableKeepAlives int32 // accessed atomically.
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

func (srv *Server) HandleAction(action uint16, handleFn handleFunc) {
	if action == ActionError {
		panic("invalid action")
	}

	srv.handlers[action] = &Handler{
		Handle: handleFn,
	}
}

func (srv *Server) ListenAndServe() error {
	var (
		ln  net.Listener
		err error
	)

	switch adr := srv.Addr.(type) {
	case *VSockAddr:
		ln, err = vsock.ListenContextID(adr.ContextId, adr.Port, nil)
	case *HttpAddr:
		ln, err = net.Listen("tcp", adr.GetAddr())
	}

	if err != nil {
		return err
	}

	return srv.Serve(ln)

}

func (srv *Server) Serve(l net.Listener) error {
	fmt.Printf("srv.Serve(%v)...\n", srv.Addr.GetAddr())
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

		fmt.Println("l.accept conn...")
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
	return atomic.LoadInt32(&srv.disableKeepAlives) == 0
}

func (srv *Server) idleTimeout() time.Duration {
	if srv.IdleTimeout != 0 {
		return srv.IdleTimeout
	}
	return srv.ReadTimeout
}
