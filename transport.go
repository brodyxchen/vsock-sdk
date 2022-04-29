package vsock

import (
	"bufio"
	"github.com/mdlayher/vsock"
	"net"
	"time"
)

const(
	maxReadBytes = 4 << 10
)


type Transport struct {
	connPool map[connectKey][]*PersistConn
	idleTimeout time.Duration

	maxIdleCount int

	// WriteBufferSize specifies the size of the write buffer used
	// when writing to the transport.
	// If zero, a default (currently 4KB) is used.
	WriteBufferSize int

	// ReadBufferSize specifies the size of the read buffer used
	// when reading from the transport.
	// If zero, a default (currently 4KB) is used.
	ReadBufferSize int
}

func (tp *Transport) getConn(addr Addr) (*PersistConn, error) {
	key := connectKey{}
	key.From(addr)

	var beginTime time.Time
	if tp.idleTimeout > 0 {
		beginTime = time.Now().Add(-tp.idleTimeout)
	}

	// 查找缓存
	var findPConn *PersistConn
	if list, ok := tp.connPool[key]; ok {
		// 倒序查找
		for len(list) > 0 {
			pConn := list[len(list)-1]

			tooOld := !beginTime.IsZero() && pConn.idleAt.Round(0).Before(beginTime)
			if tooOld {
				list = list[:len(list)-1]
				continue
			}

			findPConn = pConn
			list = list[:len(list)-1]	// 从缓存删除
			if len(list) > 0 {
				tp.connPool[key] = list
			} else {
				delete(tp.connPool, key)
			}
			return findPConn, nil
		}
	}

	// 创建
	var (
		rwConn net.Conn
		err error
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
		transport:  tp,
		connectKey: key,
		conn:       rwConn,
		reqCh:      make(chan *RequestWrapper, 1),
		writeCh:    make(chan *WriteRequest, 1),
		sawEOF:     false,
		reused:     false,
		closed:     false,
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
	pConn.reused = true

	pConn.idleAt = time.Now()
	pConn.idleTimer = time.AfterFunc(tp.idleTimeout, pConn.close)

	list, ok := tp.connPool[pConn.connectKey]
	if !ok {
		list = make([]*PersistConn, 0)
	}

	if tp.maxIdleCount > 0 && len(list) >= tp.maxIdleCount{
		cut := len(list)- tp.maxIdleCount+1
		list = list[cut:]
	}

	list = append(list, pConn)
	tp.connPool[pConn.connectKey] = list
}

func (tp *Transport) roundTrip(req *Request) (*Response, error) {
	var (
		conn *PersistConn
		err error
	)

	defer func() {
		if conn != nil && !conn.closed {
			tp.putConn(conn)
		}
	}()

	for {
		conn, err = tp.getConn(req.RemoteAddr)
		if err != nil {
			return nil, err
		}

		rsp, err := conn.roundTrip(req)
		if err == nil {
			return rsp, nil
		}

		if !conn.shouldRetryRequest(err) {
			return nil, err
		}
	}
}



func (tp *Transport) removeConn(target *PersistConn) {
	list, ok := tp.connPool[target.connectKey]
	if !ok {
		return
	}

	if target.idleTimer != nil {
		target.idleTimer.Stop()
	}

	if len(list) <= 0 {
		return
	}

	for k, conn := range list {
		if conn != target {
			continue
		}

		copy(list[k:], list[k+1:])
		tp.connPool[target.connectKey] = list[:len(list)-1]
	}
}


