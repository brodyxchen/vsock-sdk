package vsock

import (
	"bufio"
	"context"
	"io"
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

	sawEOF bool // whether we've seen EOF from conn; owned by readLoop
	//readLimit int64 // bytes allowed to be read; owned by readLoop

	nwrite int64 // bytes written

	reused bool
	closed bool
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

func (pc *PersistConn) maxHeaderResponseSize() int64 {
	//if v := pc.transport.MaxResponseHeaderBytes; v != 0 {
	//	return v
	//}
	return 10 << 20 // conservative default; same as http2
}

func (pc *PersistConn) isReused() bool {
	return pc.reused
}

func (pc *PersistConn) shouldRetryRequest(err error) bool {
	if err == nil {
		return false
	}

	return pc.isReused()
}

func (pc *PersistConn) Read(p []byte) (n int, err error) {
	n, err = pc.conn.Read(p)
	if err == io.EOF {
		pc.sawEOF = true
	}
	return
}

func (pc *PersistConn) Write(p []byte) (n int, err error) {
	n, err = pc.conn.Write(p)
	pc.nwrite += int64(n)
	return
}

func (pc *PersistConn) close() {
	pc.closed = true
	pc.transport.removeConn(pc)
	_ = pc.conn.Close()
}

func (pc *PersistConn) writeLoop() {
	defer pc.close()

	for {
		select {
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
		_, err := pc.bufReader.Peek(1)
		req := <-pc.reqCh

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
