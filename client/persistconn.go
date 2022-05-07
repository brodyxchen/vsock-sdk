package client

import (
	"bufio"
	"github.com/brodyxchen/vsock/errors"
	"github.com/brodyxchen/vsock/log"
	"github.com/brodyxchen/vsock/models"
	"github.com/brodyxchen/vsock/protocols"
	"github.com/brodyxchen/vsock/socket"
	"google.golang.org/protobuf/proto"
	"net"
	"strconv"
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
	gone := make(chan struct{})
	defer close(gone)

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

	var (
		receiveTimer <-chan time.Time
		ctxDone      = req.Ctx.Done()
	)

	for {
		select {
		case err := <-sendReply:
			if err != nil {
				return nil, err
			}
			// 发送成功，则设置读取超时
			if d := pc.transport.receiveTimeout; d > 0 {
				timer := time.NewTimer(d)
				defer timer.Stop()
				receiveTimer = timer.C
			}
		case rpy := <-receiveReply:
			return rpy.Rsp, rpy.Err

		// 异常处理
		case <-pc.closedCh: // 外部关闭
			return nil, errors.ErrConnEarlyClose
		case <-receiveTimer: // 接收超时
			return nil, errors.ErrReadTimeout
		case <-ctxDone: // ctx结束
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
	//log.Infof("persisConn[%v].idle closeAndRemove()\n", pc.Name)
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
			err := socket.WriteSocket(req.Ctx, pc.bufWriter, &req.Header, req.Body) //todo 卡死？
			if err != nil {
				writeReq.Reply <- err
				pc.close(err)
				return
			}

			writeReq.Reply <- nil
		}
	}
}

func (pc *PersistConn) readLoop() {
	closeErr := errors.New("pc.readLoop() => default exiting")
	defer func() {
		pc.close(closeErr)
	}()

	var err error

	wrap := func(header *models.Header, body []byte, err error) (*models.Response, error) {
		if err != nil {
			return nil, err
		}
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
			Header: *header,
			//Code:     0,
			//Body:     nil,
			//Err:      nil,
			Req:      nil,
			ConnName: pc.Name,
		}

		// 业务错误
		if pbBody.Code != 0 {
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

	var notifyReq *models.NotifyReceive
	for !pc.isClosed() {
		_, err = pc.bufReader.Peek(1) // 阻塞		//todo 卡死？
		if err != nil {
			//errMsg := err.Error()
			//todo client 这里isClosed标志没有关闭， 但是conn关闭了
			// 可能1. 其他go关闭， 2. 对手关闭,  3. 中间网络关闭

			//isClose := pc.isClosed()
			//if !isClose {
			//	fmt.Printf("persisConn[%v].readLoop[%v] Peek err: %v\n", pc.Name, pc.isClosed(), errMsg)	//todo true read tcp 127.0.0.1:52492->127.0.0.1:7070: use of closed network connection
			//}

			//fmt.Println(pc.isClosed(), errMsg)
			closeErr = errors.New("readLoop() peek err => " + err.Error())
			return
		}

		notifyReq = <-pc.receiveCh

		keyName := pc.transport.Name + "--" + strconv.FormatInt(pc.Name, 10)
		header, body, err := socket.ReadSocket(keyName, notifyReq.Req.Ctx, pc.bufReader) //todo 卡死？// 这里err是系统错误，不是业务错误

		rsp, sysErr := wrap(header, body, err)
		if rsp != nil {
			rsp.Req = notifyReq.Req
		}

		select {
		case notifyReq.Reply <- &models.ReceiveResponse{Rsp: rsp, Err: sysErr}:
		case <-notifyReq.CallerGone:
			closeErr = errors.New("readLoop() return => caller gone")
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
