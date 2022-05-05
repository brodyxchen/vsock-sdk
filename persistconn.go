package vsock

import (
	"bufio"
	"context"
	"net"
	"time"
)

type PersistConn struct {
	transport  *Transport
	connectKey connectKey

	conn      net.Conn
	bufReader *bufio.Reader // from conn
	bufWriter *bufio.Writer // to conn

	reqCh   chan *RequestWrapper
	writeCh chan *WriteRequest

	// Both guarded by Transport.idleMu:
	idleAt    time.Time   // time it last become idle
	idleTimer *time.Timer // holding an AfterFunc to close it

	state  ConnState
	reused bool

	closed   bool
	closedCh chan struct{}
}

func (pc *PersistConn) roundTrip(req *Request) (*Response, error) {
	gone := make(chan struct{})
	defer close(gone)

	// 发送数据
	writeReply := make(chan error, 1)
	pc.writeCh <- &WriteRequest{Req: req, Reply: writeReply}

	// 通知接收数据
	readReply := make(chan *ReadResponse, 1)
	pc.reqCh <- &RequestWrapper{
		Req:        req,
		Reply:      readReply,
		callerGone: gone,
	}

	for {
		select {
		case err := <-writeReply:
			if err != nil {
				return nil, err
			}
		case rpy := <-readReply:
			return rpy.Rsp, rpy.Err
		}
	}
}

func (pc *PersistConn) Read(p []byte) (n int, err error) {
	n, err = pc.conn.Read(p)
	return
}

func (pc *PersistConn) Write(p []byte) (n int, err error) {
	n, err = pc.conn.Write(p)
	return
}

func (pc *PersistConn) closeIfIdle() {
	if pc.state != ConnStateIdle {
		return
	}

	pc.close()
}

func (pc *PersistConn) close() {
	if pc.closed {
		return
	}

	pc.closed = true
	close(pc.closedCh)

	pc.state = ConnStateIdle

	if pc.idleTimer != nil {
		pc.idleTimer.Stop()
	}
	pc.transport.removeConn(pc)

	_ = pc.conn.Close()
}

func (pc *PersistConn) writeLoop() {
	defer pc.close()

	for {
		select {
		case <-pc.closedCh:
		case writeReq := <-pc.writeCh:
			ctx := context.Background()

			req := writeReq.Req
			err := writeSocket(ctx, pc.bufWriter, &req.Header, req.Body)
			if err != nil {
				writeReq.Reply <- err
				return
			}

			writeReq.Reply <- nil
		}
	}
}

func (pc *PersistConn) readLoop() {
	defer pc.close()

	for {
		ctx := context.Background()
		//_, err := pc.bufReader.Peek(1)
		var req *RequestWrapper
		select {
		case <-pc.closedCh:
			return
		case req = <-pc.reqCh:
		}

		var rsp Response
		header, body, err := readSocket(ctx, pc.bufReader)
		if err == nil {
			rsp.Header = *header
			rsp.Body = body
		}

		select {
		case req.Reply <- &ReadResponse{Rsp: &rsp, Err: err}:
			// 响应数据
		case <-req.callerGone:
			// 没有调用者
		}
	}
}
