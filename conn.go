package vsock

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"runtime"
	"time"
)

type Conn struct {
	server *Server

	cancelCtx context.CancelFunc //todo 没用到

	rwc net.Conn

	remoteAddr string

	bufReader *bufio.Reader
	bufWriter *bufio.Writer
}

func (c *Conn) Read(p []byte) (n int, err error) {
	return c.rwc.Read(p)
}

func (c *Conn) Write(p []byte) (n int, err error) {
	return c.rwc.Write(p)
}

func (c *Conn) handleServe(ctx context.Context, header *sockHeader, body []byte) ([]byte, bool) {
	handler, ok := c.server.handlers[header.Code]
	if !ok {
		return nil, false
	}

	rspBytes := handler.Handle(header.Code, body)

	return rspBytes, true
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

	for {
		now := time.Now()

		// 读取req
		if c.server.ReadTimeout != 0 {
			_ = c.rwc.SetReadDeadline(now.Add(c.server.ReadTimeout))
		}
		header, body, err := readSocket(ctx, c.bufReader)
		if err != nil {
			//if err == os.ErrDeadlineExceeded {
			//}
			c.Close(err)
			return
		}
		fmt.Printf("readSocket : %+v\n", header)

		if c.server.WriteTimeout != 0 {
			_ = c.rwc.SetWriteDeadline(time.Now().Add(c.server.WriteTimeout))
		}

		// handle
		rspBytes, ok := c.handleServe(ctx, header, body)
		if !ok {
			err = c.responseError(ctx, CodeInvalidAction, "invalid action")
			if err != nil {
				c.Close(err)
				return
			}
		}

		// 响应rsp
		header.Code = 0
		header.Length = uint16(len(rspBytes))
		err = writeSocket(ctx, c.bufWriter, header, rspBytes)
		if err != nil {
			c.Close(err)
			return
		}

		if !c.server.doKeepAlives() {
			return
		}

		// 阻塞等待 下一份数据
		if wait := c.server.idleTimeout(); wait != 0 {
			_ = c.rwc.SetReadDeadline(time.Now().Add(wait))
			if _, err := c.bufReader.Peek(HeaderSize); err != nil {
				c.Close(err)
				return
			}
			fmt.Println("Peek new bytes")
		}
		_ = c.rwc.SetReadDeadline(time.Time{})
	}
}

func readSocket(ctx context.Context, reader *bufio.Reader) (*sockHeader, []byte, error) {
	header := &sockHeader{}

	headerBuf := make([]byte, HeaderSize)
	n, err := io.ReadFull(reader, headerBuf)
	if err != nil {
		if err == io.EOF {
			fmt.Printf("readSocket() err == io.EOF, n=%v\n", n)
			return nil, nil, errInvalidHeader
		}
		fmt.Printf("readSocket() err != nil: %v\n", err)
		return nil, nil, errInvalidHeader
	}
	if n < HeaderSize {
		fmt.Printf("readSocket() n(%v) < HeaderSize\n", n)
		return nil, nil, errInvalidHeader
	}

	header.Magic = binary.BigEndian.Uint16(headerBuf[:])
	if header.Magic != defaultMagic {
		fmt.Printf("readSocket() header.Magic(%v) != defaultMagic\n", header.Magic)
		return nil, nil, errInvalidHeader
	}

	header.Version = binary.BigEndian.Uint16(headerBuf[2:])
	header.Code = binary.BigEndian.Uint16(headerBuf[4:])
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

func writeSocket(ctx context.Context, writer *bufio.Writer, header *sockHeader, body []byte) error {
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
	binary.BigEndian.PutUint16(buf[4:], header.Code)
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

func (c *Conn) responseError(ctx context.Context, code uint16, msg string) error {
	fmt.Println("responseError : ", msg)

	header := &sockHeader{
		Magic:   defaultMagic,
		Version: defaultVersion,
		Code:    code,
		Length:  0,
	}
	body := []byte(msg)
	header.Length = uint16(len(body))

	return writeSocket(ctx, c.bufWriter, header, body)
}

func (c *Conn) Close(err error) {
	fmt.Println("Close : ", err.Error())

	c.cancelCtx()
	_ = c.rwc.Close()

	putBufReader(c.bufReader)
	putBufWriter(c.bufWriter)
}
