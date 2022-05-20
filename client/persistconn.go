package client

import (
	"bufio"
	"github.com/brodyxchen/vsock/errors"
	"github.com/brodyxchen/vsock/log"
	"github.com/brodyxchen/vsock/models"
	"github.com/brodyxchen/vsock/protocols"
	"github.com/brodyxchen/vsock/socket"
	"google.golang.org/protobuf/proto"
	"io"
	"net"
	"sync"
	"time"
)

type PersistConn struct {
	Name int64

	key       connectKey
	transport *Transport

	conn      net.Conn
	bufReader *bufio.Reader // from conn
	bufWriter *bufio.Writer // to conn

	receiveCh chan *models.NotifyReceive
	sendCh    chan *models.SendRequest

	// 以下三个变量被connPool.mutex守护
	idleAt    time.Time   // time it last become idle
	idleTimer *time.Timer // holding an AfterFunc to close it
	reused    bool

	closedMutex sync.RWMutex // 守护以下2个变量
	closed      error
	closedCh    chan struct{}
}

// roundTrip 一次往返，不处理关闭和链接池， 由上层transport处理
func (pc *PersistConn) roundTrip(req *models.Request) (*models.Response, error) {
	gone := make(chan struct{}) //todo 多余的不？
	defer close(gone)

	sendNow := time.Now()

	// 发送数据
	sendReply := make(chan error, 1)
	pc.sendCh <- &models.SendRequest{Req: req, Reply: sendReply}

	// 通知接收
	receiveReply := make(chan *models.ReceiveResponse, 1)
	pc.receiveCh <- &models.NotifyReceive{
		Req:        req,
		Reply:      receiveReply,
		CallerGone: gone,
	}

	for {
		select {
		case err := <-sendReply:
			pc.transport.sendDoneHist.Update(time.Since(sendNow).Milliseconds())
			if err != nil {
				return nil, err
			}
		case rpy := <-receiveReply:
			pc.transport.receiveHist.Update(time.Since(sendNow).Milliseconds())
			return rpy.Rsp, rpy.Err

		// 异常处理
		case <-pc.closedCh: // 外部关闭
			pc.transport.receiveTimeoutHist.Update(time.Since(sendNow).Milliseconds())
			return nil, errors.ErrConnEarlyClose
		case <-req.Ctx.Done(): // ctx结束
			pc.transport.receiveTimeoutHist.Update(time.Since(sendNow).Milliseconds())
			return nil, errors.ErrCtxRoundDone
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

func (pc *PersistConn) closeAndRemove() {
	if !pc.transport.removeConn(pc) {
		return
	}
	pc.close(errors.New("pc.closeAndRemove() => idle timer close"))
}

func (pc *PersistConn) writeLoop() {
	for {
		select {
		case <-pc.closedCh:
			return
		case writeReq := <-pc.sendCh:
			req := writeReq.Req
			broken, err := socket.WriteSocket(req.Ctx, pc.bufWriter, &req.Header, req.Body)
			if err != nil {
				writeReq.Reply <- err

				if broken {
					pc.close(err)
					return
				}
			} else {
				writeReq.Reply <- nil
			}
		}
	}
}

func (pc *PersistConn) readLoop() {
	closeErr := errors.New("pc.readLoop() => default exiting")

	defer func() {
		pc.close(closeErr)
	}()

	var err error

	wrap := func(header *models.Header, body []byte) (*models.Response, error) {
		if header.Length == 0 || len(body) <= 0 {
			return nil, errors.ErrUnknownServerErr
		}

		// 服务器 错误
		if header.Code != 0 {
			errMsg := string(body)
			return nil, errors.New(errMsg)
		}

		var pbBody protocols.Response
		err = proto.Unmarshal(body, &pbBody)
		if err != nil {
			return nil, err
		}

		rsp := &models.Response{
			Header:   *header,
			Req:      nil,
			ConnName: pc.Name,
		}

		// 业务错误
		if pbBody.Code != protocols.StatusOK {
			rsp.Code = uint16(pbBody.Code)
			rsp.Body = nil
			rsp.Err = errors.New(pbBody.Err)
		} else {
			rsp.Code = 0
			rsp.Body = pbBody.Rsp
			rsp.Err = nil
		}

		return rsp, nil
	}

	var (
		notifyReq *models.NotifyReceive
		rsp       *models.Response
	)
	for !pc.isClosed() {
		_, err = pc.bufReader.Peek(1) // 阻塞
		if err != nil {
			if err == io.EOF {
				continue
			}
			closeErr = errors.New("readLoop() peek err => " + err.Error())
			return
		}

		notifyReq = <-pc.receiveCh

		header, body, broken, err := socket.ReadSocket(notifyReq.Req.Ctx, pc.bufReader)
		if err == nil {
			rsp, err = wrap(header, body)
			if rsp != nil {
				rsp.Req = notifyReq.Req
			}
		} else {
			rsp = nil
		}

		select {
		case notifyReq.Reply <- &models.ReceiveResponse{Rsp: rsp, Err: err}:
		case <-notifyReq.CallerGone:
		}

		if err != nil && broken {
			closeErr = err
			return
		}
	}
}

func (pc *PersistConn) isClosed() bool {
	pc.closedMutex.RLock()
	defer pc.closedMutex.RUnlock()
	return pc.closed != nil
}
func (pc *PersistConn) close(err error) {
	if err == nil {
		panic("close with nil err")
	}
	pc.closedMutex.Lock()
	defer pc.closedMutex.Unlock()
	pc.closeLocked(err)
}

func (pc *PersistConn) closeLocked(err error) {
	if pc.closed != nil {
		return
	}
	log.Debugf("persisConn[%v].closeLocked() : %v\n", pc.Name, err)

	pc.closed = err
	close(pc.closedCh)

	if pc.idleTimer != nil {
		pc.idleTimer.Stop()
	}

	_ = pc.conn.Close()
}
func (pc *PersistConn) CloseTest() {
	pc.closedMutex.Lock()
	defer pc.closedMutex.Unlock()
	pc.closeLocked(io.EOF)
}
