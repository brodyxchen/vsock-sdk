package client

import (
	"bufio"
	"github.com/brodyxchen/vsock/constant"
	"github.com/brodyxchen/vsock/log"
	"github.com/brodyxchen/vsock/models"
	"github.com/mdlayher/vsock"
	"net"
	"sync/atomic"
	"time"
)

const (
	maxRetryCount = 1
)

type Transport struct {
	Name     string
	connPool ConnPool

	sendTimeout    time.Duration
	receiveTimeout time.Duration

	WriteBufferSize int
	ReadBufferSize  int

	connIndex int64 // atomic visit
}

func (tp *Transport) getConnIndex() int64 {
	value := atomic.AddInt64(&tp.connIndex, 1)
	return value
}

func (tp *Transport) getConn(addr models.Addr, retryCount int) (*PersistConn, error) {
	key := connectKey{}
	key.From(addr)

	if retryCount <= 0 {
		// 查找缓存
		findConn := tp.connPool.Get(key)
		if findConn != nil {
			return findConn, nil
		}
	}

	// 创建
	var (
		rwConn net.Conn
		err    error
	)
	switch ad := addr.(type) {
	case *models.VSockAddr:
		rwConn, err = vsock.Dial(ad.ContextId, ad.Port, nil)
		if err != nil {
			return nil, err
		}
	case *models.HttpAddr:
		rwConn, err = net.Dial("tcp", ad.GetAddr())
		if err != nil {
			return nil, err
		}
	default:
		panic("invalid models addr")
	}

	pConn := &PersistConn{
		Name:      tp.getConnIndex(),
		key:       key,
		transport: tp,
		conn:      rwConn,
		receiveCh: make(chan *models.NotifyReceive, 1),
		sendCh:    make(chan *models.SendRequest, 1),
		idleAt:    time.Time{},
		idleTimer: nil,
		reused:    false,
		closed:    false,
		closedCh:  make(chan struct{}, 1),
	}
	pConn.bufReader = bufio.NewReaderSize(pConn, tp.readBufferSize())
	pConn.bufWriter = bufio.NewWriterSize(pConn, tp.writeBufferSize())

	go pConn.readLoop()
	go pConn.writeLoop()

	log.Info("create conn ", tp.Name, pConn.Name)
	return pConn, nil
}

func (tp *Transport) writeBufferSize() int {
	if tp.WriteBufferSize > 0 {
		return tp.WriteBufferSize
	}
	return constant.MaxWriteBufferSize
}

func (tp *Transport) readBufferSize() int {
	if tp.ReadBufferSize > 0 {
		return tp.ReadBufferSize
	}
	return constant.MaxReadBufferSize
}

func (tp *Transport) putConn(pConn *PersistConn) {
	tp.connPool.Put(pConn)
}

func (tp *Transport) roundTrip(req *Request) (*Response, error) {
	var (
		ctx        = req.Context()
		retryCount = 0
		conn       *PersistConn
		err        error
		sRsp       *models.Response
	)

	closeConn := func(pConn *PersistConn) {
		if pConn == nil {
			return
		}
		pConn.close()
	}

	defer func() {
		if err == nil && conn != nil && !conn.closed {
			tp.putConn(conn)
			return
		}

		// 关闭conn
		closeConn(conn)
		conn = nil
	}()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		conn, err = tp.getConn(req.Addr, retryCount)
		if err != nil {
			return nil, err
		}

		sReq := &models.Request{
			Ctx: ctx,
			Header: models.Header{
				Magic:   constant.DefaultMagic,
				Version: constant.DefaultVersion,
				Code:    req.Action,
				Length:  uint16(len(req.Body)),
			},
			Body: req.Body,
		}

		sRsp, err = conn.roundTrip(sReq)
		if err == nil {
			return &Response{
				Req:      req,
				ConnName: conn.Name,
				Code:     sRsp.Code,
				Body:     sRsp.Body,
			}, nil
		}

		// 是否重试
		if !conn.reused || retryCount > maxRetryCount {
			return nil, err
		}

		// 准备重试
		retryCount++
		closeConn(conn)
		conn = nil
	}
}

func (tp *Transport) removeConn(target *PersistConn) {
	tp.connPool.Remove(target)
}
