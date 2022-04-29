package vsock

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mdlayher/vsock"
	"io"
	"log"
	"net"
	"runtime"
	"time"
)

type handleFunc func(net.Conn, []byte) ([]byte, error)

func NewServer(addr Addr) *Server {
	return &Server{
		Addr:              addr,
		handlers:          make(map[string]handleFunc, 0),
		ReadTimeout:       0,
		WriteTimeout:      0,
		IdleTimeout:       time.Hour,
		disableKeepAlives: 0,
	}
}

type Server struct {
	Addr Addr

	handlers map[string]handleFunc

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

func (srv *Server) HandleAction(action string, handleFn handleFunc) {
	srv.handlers[action] = handleFn
}

func (srv *Server) ListenAndServe() error {
	var (
		ln net.Listener
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

type Conn struct {
	server *Server

	cancelCtx context.CancelFunc

	rwc net.Conn

	remoteAddr string

	bufReader *bufio.Reader
	bufWriter *bufio.Writer

	isHijacked bool
}

func (c *Conn) Read(p []byte) (n int, err error) {
	return c.rwc.Read(p)
}

func (c *Conn) Write(p []byte) (n int, err error) {
	return c.rwc.Write(p)
}

func (c *Conn) hijacked() bool {
	return c.isHijacked
}

// Serve a new connection.
func (c *Conn) serve(ctx context.Context) {
	c.remoteAddr = c.rwc.RemoteAddr().String()

	ctx = context.Background()
	defer func() {
		if err := recover(); err != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			log.Printf("http: panic serving %v: %v\n%s", c.remoteAddr, err, buf)
		}
	}()

	ctx, cancelCtx := context.WithCancel(ctx)
	c.cancelCtx = cancelCtx
	defer cancelCtx()

	c.bufReader = getBufReader(c)
	c.bufWriter = getBufWriter(c)

	if c.server.ReadTimeout > 0 {
		_ = c.rwc.SetReadDeadline(time.Now().Add(c.server.ReadTimeout))
	} else {
		_ = c.rwc.SetReadDeadline(time.Time{})
	}

	// server读取数据
	reqBytes, err := c.readRequest(ctx)
	if err != nil {
		c.responseError(ctx, err)
		return
	}

	var req Request
	err = json.Unmarshal(reqBytes, &req)
	if err != nil {
		c.responseError(ctx, err)
		return
	}
	data, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		c.responseError(ctx, err)
		return
	}

	// handler
	handle, ok := c.server.handlers[req.Action]
	if !ok {
		err = errors.New("invalid action")
		c.responseError(ctx, err)
		return
	}
	outBytes, err := handle(c.rwc, data)
	if err != nil {
		c.responseError(ctx, err)
		return
	}

	rsp := &Response{
		Request: &req,		// 关联req
		Result:  "ok",
		Data:    base64.StdEncoding.EncodeToString(outBytes),
	}

	rspBytes, err := json.Marshal(rsp)
	if err != nil {
		c.responseError(ctx, err)
		return
	}

	err = c.writeResponse(ctx, rspBytes)
	if err != nil {
		return
	}

	_ = c.rwc.SetReadDeadline(time.Time{})
}

func (c *Conn) readRequest(ctx context.Context) ([]byte, error) {
	var dataBuf bytes.Buffer

	buf := make([]byte, maxReadBytes)
	for {
		n, err := c.bufReader.Read(buf)
		if err != nil {
			if err == io.EOF {
				return dataBuf.Bytes(), nil
			}
			return nil, err
		}
		dataBuf.Write(buf[:n])
	}
}

func (c *Conn) writeResponse(ctx context.Context, bytes []byte) error {
	if c.server.WriteTimeout > 0 {
		_ = c.rwc.SetWriteDeadline(time.Now().Add(c.server.WriteTimeout))
	} else {
		_ = c.rwc.SetWriteDeadline(time.Time{})
	}

	_, err := c.bufWriter.Write(bytes)

	return err
}


func (c *Conn) responseError(ctx context.Context, err error) {
	rsp := &Response{
		Result:  "error",
		Data:    err.Error(),
	}

	rspBytes, _ := json.Marshal(rsp)
	err = c.writeResponse(ctx, rspBytes)
}