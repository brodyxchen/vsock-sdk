package client

import (
	"bufio"
	"cryptobroker/vsock-sdk/constant"
	"cryptobroker/vsock-sdk/errors"
	"cryptobroker/vsock-sdk/log"
	"cryptobroker/vsock-sdk/models"
	"cryptobroker/vsock-sdk/statistics/metrics"
	"github.com/mdlayher/vsock"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxRetryCount = 1
)

type Transport struct {
	Name     string
	connPool ConnPool

	WriteBufferSize int
	ReadBufferSize  int

	connIndex int64 // atomic visit

	connGetHist metrics.Histogram
	connNewHist metrics.Histogram
	tripHist    metrics.Histogram

	sendHist           metrics.Histogram
	sendDoneHist       metrics.Histogram
	receiveHist        metrics.Histogram
	receiveTimeoutHist metrics.Histogram
}

func (tp *Transport) getConnIndex() int64 {
	value := atomic.AddInt64(&tp.connIndex, 1)
	return value
}

func (tp *Transport) DialTest(addr models.Addr) (*PersistConn, error) {

	key := connectKey{}
	key.From(addr)

	// 创建
	var (
		rwConn net.Conn
		err    error
	)

	now := time.Now()
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
	tp.connNewHist.Update(time.Since(now).Milliseconds())

	pConn := &PersistConn{
		Name:        tp.getConnIndex(),
		key:         key,
		transport:   tp,
		conn:        rwConn,
		receiveCh:   make(chan *models.NotifyReceive, 1),
		sendCh:      make(chan *models.SendRequest, 1),
		idleAt:      time.Time{},
		idleTimer:   nil,
		reused:      false,
		closedMutex: sync.RWMutex{},
		closed:      nil,
		closedCh:    make(chan struct{}, 1),
	}
	pConn.bufReader = bufio.NewReaderSize(pConn, tp.readBufferSize())
	pConn.bufWriter = bufio.NewWriterSize(pConn, tp.writeBufferSize())

	go pConn.readLoop()
	go pConn.writeLoop()

	log.Debug("create conn ", tp.Name, pConn.Name)

	return pConn, nil
}

func (tp *Transport) getConn(addr models.Addr, retryCount int) (*PersistConn, error) {
	now := time.Now()

	key := connectKey{}
	key.From(addr)

	if retryCount <= 0 {
		// 查找缓存
		findConn := tp.connPool.Get(key)
		if findConn != nil {
			tp.connGetHist.Update(time.Since(now).Milliseconds())
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
		Name:        tp.getConnIndex(),
		key:         key,
		transport:   tp,
		conn:        rwConn,
		receiveCh:   make(chan *models.NotifyReceive, 1),
		sendCh:      make(chan *models.SendRequest, 1),
		idleAt:      time.Time{},
		idleTimer:   nil,
		reused:      false,
		closedMutex: sync.RWMutex{},
		closed:      nil,
		closedCh:    make(chan struct{}, 1),
	}
	pConn.bufReader = bufio.NewReaderSize(pConn, tp.readBufferSize())
	pConn.bufWriter = bufio.NewWriterSize(pConn, tp.writeBufferSize())

	go pConn.readLoop()
	go pConn.writeLoop()

	log.Debug("create conn ", tp.Name, pConn.Name)

	tp.connNewHist.Update(time.Since(now).Milliseconds())
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

func (tp *Transport) roundTrip(req *models.Request) (*models.Response, error) {
	var (
		ctx        = req.Context()
		retryCount = 0
		conn       *PersistConn
		err        error
		sRsp       *models.Response
	)

	closeConn := func(pConn *PersistConn, err error) {
		if pConn == nil {
			return
		}
		pConn.close(err)
	}

	defer func() {
		if err == nil && conn != nil && !conn.isClosed() {
			tp.putConn(conn)
			return
		}

		// 关闭conn
		if err == nil {
			if conn.closed == nil {
				closeConn(conn, errors.ErrTransportTripClose)
			}
		} else {
			closeConn(conn, err)
		}
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
				Code:    0, // 一些特殊设置: 比如keepAlive
				Length:  uint16(len(req.Body)),
			},
			Body: req.Body,
		}

		tripNow := time.Now()
		sRsp, err = conn.roundTrip(sReq)
		tp.tripHist.Update(time.Since(tripNow).Milliseconds())

		if err == nil {
			return sRsp, nil
		}

		// 是否重试	//FIXME 对于tempErr进行重试
		if !conn.reused || retryCount > maxRetryCount {
			return nil, err
		}

		// 准备重试
		retryCount++
		closeConn(conn, err)
		conn = nil
	}
}

func (tp *Transport) removeConn(target *PersistConn) bool {
	return tp.connPool.Remove(target)
}
