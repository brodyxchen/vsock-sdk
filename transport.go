package vsock

import (
	"bufio"
	"github.com/mdlayher/vsock"
	"net"
	"sync/atomic"
	"time"
)

const (
	maxReadBytes   = 4 << 10
	defaultMagic   = uint16(0x1617)
	defaultVersion = uint16(1)

	maxRetryCount = 1
)

type Transport struct {
	connPool ConnPool

	idleTimeout time.Duration

	// WriteBufferSize specifies the size of the write buffer used
	// when writing to the transport.
	// If zero, a default (currently 4KB) is used.
	WriteBufferSize int

	// ReadBufferSize specifies the size of the read buffer used
	// when reading from the transport.
	// If zero, a default (currently 4KB) is used.
	ReadBufferSize int

	// 递增的，记录conn.name
	connIndex int64
}

func (tp *Transport) getConnIndex() int64 {
	value := atomic.AddInt64(&tp.connIndex, 1)
	return value
}

func (tp *Transport) getConn(addr Addr, retryCount int) (*PersistConn, error) {
	key := connectKey{}
	key.From(addr)

	var beginTime time.Time
	if tp.idleTimeout > 0 {
		beginTime = time.Now().Add(-tp.idleTimeout)
	}

	if retryCount <= 0 {
		// 查找缓存
		findConn := tp.connPool.Get(key, beginTime)
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
	case *VSockAddr:
		rwConn, err = vsock.Dial(ad.ContextId, ad.Port, nil)
		if err != nil {
			return nil, err
		}
	case *HttpAddr:
		rwConn, err = net.Dial("tcp", ad.GetAddr())
		if err != nil {
			return nil, err
		}
	}

	pConn := &PersistConn{
		Name:       tp.getConnIndex(),
		transport:  tp,
		connectKey: key,
		conn:       rwConn,
		reqCh:      make(chan *RequestWrapper, 1),
		writeCh:    make(chan *WriteRequest, 1),
		idleAt:     time.Time{},
		idleTimer:  nil,
		state:      ConnStateBusy,
		reused:     false,
		closed:     false,
		closedCh:   make(chan struct{}, 1),
	}
	pConn.bufReader = bufio.NewReaderSize(pConn, tp.readBufferSize())
	pConn.bufWriter = bufio.NewWriterSize(pConn, tp.writeBufferSize())

	go pConn.readLoop()
	go pConn.writeLoop()

	return pConn, nil
}

func (tp *Transport) writeBufferSize() int {
	if tp.WriteBufferSize > 0 {
		return tp.WriteBufferSize
	}
	return 4 << 10
}

func (tp *Transport) readBufferSize() int {
	if tp.ReadBufferSize > 0 {
		return tp.ReadBufferSize
	}
	return 4 << 10
}

func (tp *Transport) putConn(pConn *PersistConn) {
	tp.connPool.Put(pConn, tp.idleTimeout)
}

func (tp *Transport) roundTrip(req *Request) ([]byte, error) {
	var (
		ctx        = req.Context()
		retryCount = 0
		conn       *PersistConn
		err        error
	)

	defer func() {
		if conn != nil && !conn.closed {
			tp.putConn(conn)
		}
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

		sReq := &sockRequest{
			ctx: ctx,
			sockHeader: sockHeader{
				Magic:   defaultMagic,
				Version: defaultVersion,
				Code:    req.Action,
				Length:  uint16(len(req.Body)),
			},
			Body: req.Body,
		}

		rsp, err := conn.roundTrip(sReq)
		if err == nil {
			return rsp.Body, nil
		}

		// 是否重试
		if !conn.reused || retryCount > maxRetryCount {
			return nil, err
		}

		retryCount++
	}
}

func (tp *Transport) roundTripTest(addr Addr, action uint16, reqBytes []byte) (int64, []byte, error) {
	var (
		retryCount = 0
		conn       *PersistConn
		err        error
	)

	defer func() {
		if conn != nil && !conn.closed {
			tp.putConn(conn)
		}
	}()

	for {
		conn, err = tp.getConn(addr, retryCount)
		if err != nil {
			return -1, nil, err
		}

		req := &sockRequest{
			sockHeader: sockHeader{
				Magic:   defaultMagic,
				Version: defaultVersion,
				Code:    action,
				Length:  uint16(len(reqBytes)),
			},
			Body: reqBytes,
		}

		rsp, err := conn.roundTrip(req)
		if err == nil {
			return conn.Name, rsp.Body, nil
		}

		// 是否重试
		if !conn.reused || retryCount > maxRetryCount {
			return conn.Name, nil, err
		}

		retryCount++
	}
}

func (tp *Transport) removeConn(target *PersistConn) {
	tp.connPool.Remove(target)
}
