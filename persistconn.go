package vsock

import (
	"bufio"
	"encoding/json"
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

	sawEOF    bool  // whether we've seen EOF from conn; owned by readLoop
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
	pc.writeCh <- &WriteRequest{Request: req, Reply: writeReply}

	// 通知接收数据
	readReply := make(chan *ReadResponse, 1)
	pc.reqCh <- &RequestWrapper{
		Request:    req,
		Reply:      readReply,
		callerGone: gone,
	}

	for {
		select {
		case err := <-writeReply:
			if err != nil {
				return nil, err
			}
		case rsp := <-readReply:
			return rsp.Response, rsp.Err
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
		case req := <-pc.writeCh:
			reqBytes, err := json.Marshal(req.Request)
			if err != nil {
				req.Reply <- err
				continue
			}

			_, err = pc.bufWriter.Write(reqBytes)
			if err != nil {
				req.Reply <- err
				return
			}

			err = pc.bufWriter.Flush()
			if err != nil {
				req.Reply <- err
				return
			}

			req.Reply <- nil
		}
	}
}


func (pc *PersistConn) readLoop() {
	defer pc.close()

	for {
		_, err := pc.bufReader.Peek(1)
		req := <-pc.reqCh

		buf := make([]byte, 0, maxReadBytes)
		n, err := pc.bufReader.Read(buf)
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return
		}

		data := buf[:n]

		var rsp Response
		err = json.Unmarshal(data, &rsp)

		select {
		case req.Reply <- &ReadResponse{Response: &rsp, Err: err}:
			// 响应数据
		case <-req.callerGone:
			// 没有调用者
		}
	}
}
