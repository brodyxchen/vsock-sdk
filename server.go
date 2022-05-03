package vsock

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/brodyxchen/vsock/pb"
	"github.com/mdlayher/vsock"
	"google.golang.org/protobuf/proto"
	"io"
	"math"
	"net"
	"runtime"
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
			fmt.Printf("http: panic serving %v: %v\n%s", c.remoteAddr, err, buf)
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

	// 读取req
	header, body, err := readSocket(ctx, c.bufReader)
	if err != nil {
		c.Close(err)
		return
	}
	fmt.Printf("readSocket : %+v\n", header)

	// handler
	handler, ok := c.server.handlers[header.Action]
	if !ok {
		err = errors.New("invalid action")
		err = c.responseError(ctx, header, err)
		if err != nil {
			c.Close(err)
		}
		return
	}
	// handle
	rspBytes, err := handler.Handle(header.Action, body)
	if err != nil {
		err = c.responseError(ctx, header, err)
		if err != nil {
			c.Close(err)
		}
		return
	}

	// 响应rsp
	err = writeSocket(ctx, c.bufWriter, header, rspBytes)
	if err != nil {
		c.Close(err)
		return
	}

	_ = c.rwc.SetReadDeadline(time.Time{})
}

func readSocket(ctx context.Context, reader *bufio.Reader) (*Header, []byte, error) {
	header := &Header{}

	headerBuf := make([]byte, HeaderSize)
	n, err := io.ReadFull(reader, headerBuf)
	if err != nil {
		if err == io.EOF {
			return nil, nil, errInvalidHeader
		}
		return nil, nil, errInvalidHeader
	}
	if n < HeaderSize {
		return nil, nil, errInvalidHeader
	}

	header.Magic = binary.BigEndian.Uint16(headerBuf[:])
	if header.Magic != defaultMagic {
		return nil, nil, errInvalidHeader
	}

	header.Version = binary.BigEndian.Uint16(headerBuf[2:])
	header.Action = binary.BigEndian.Uint16(headerBuf[4:])
	header.Length = binary.BigEndian.Uint16(headerBuf[6:])

	if header.Length <= 0 {
		return header, nil, nil
	}

	bodyBuf := make([]byte, header.Length)
	n, err = io.ReadFull(reader, bodyBuf) // 第一次读取27, 循环第二次读取卡住了
	if err == nil || err == io.EOF {
		if n < int(header.Length) {
			return header, nil, errInvalidBody
		}
		return header, bodyBuf[:header.Length], nil
	}

	return header, nil, errInvalidBody
}

func writeSocket(ctx context.Context, writer *bufio.Writer, header *Header, body []byte) error {
	//if c.server.WriteTimeout > 0 {
	//	_ = c.rwc.SetWriteDeadline(time.Now().Add(c.server.WriteTimeout))
	//} else {
	//	_ = c.rwc.SetWriteDeadline(time.Time{})
	//}

	length := len(body)
	if length > math.MaxUint16 {
		return errExceedBody
	}
	header.Length = uint16(length)

	buf := make([]byte, HeaderSize+length)
	binary.BigEndian.PutUint16(buf, header.Magic)
	binary.BigEndian.PutUint16(buf[2:], header.Version)
	binary.BigEndian.PutUint16(buf[4:], header.Action)
	binary.BigEndian.PutUint16(buf[6:], header.Length)
	if length > 0 {
		copy(buf[HeaderSize:], body)
	}

	_, err := writer.Write(buf)
	if err != nil {
		return err
	}

	err = writer.Flush()
	if err != nil {
		return err
	}

	return nil
}

func (c *Conn) responseError(ctx context.Context, header *Header, err error) error {
	fmt.Println("responseError : ", err.Error())

	rsp := &pb.ErrorBody{
		Code:    -1,
		Message: err.Error(),
	}

	body, _ := proto.Marshal(rsp)

	return writeSocket(ctx, c.bufWriter, header, body)
}
func (c *Conn) Close(err error) {
	fmt.Println("Close : ", err.Error())
	_ = c.rwc.Close()
}
