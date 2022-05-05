package vsock

import (
	"bufio"
	"github.com/mdlayher/vsock"
	"net"
	"time"
)

const (
	maxReadBytes   = 4 << 10
	defaultMagic   = uint16(0x1617)
	defaultVersion = uint16(1)
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
}

func (tp *Transport) getConn(addr Addr) (*PersistConn, error) {
	key := connectKey{}
	key.From(addr)

	var beginTime time.Time
	if tp.idleTimeout > 0 {
		beginTime = time.Now().Add(-tp.idleTimeout)
	}

	// 查找缓存
	findConn := tp.connPool.Get(key, beginTime)
	if findConn != nil {
		return findConn, nil
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

func (tp *Transport) roundTrip(addr Addr, action uint16, reqBytes []byte) ([]byte, error) {
	var (
		conn *PersistConn
		err  error
	)

	defer func() {
		if conn != nil && !conn.closed {
			tp.putConn(conn)
		}
	}()

	for {
		//todo 第一次 获取旧的， 第二次获取新的？
		conn, err = tp.getConn(addr)
		if err != nil {
			return nil, err
		}

		req := &Request{
			Header: Header{
				Magic:   defaultMagic,
				Version: defaultVersion,
				Action:  action,
				Length:  uint16(len(reqBytes)),
			},
			Body: reqBytes,
		}

		rsp, err := conn.roundTrip(req)
		if err == nil {
			return rsp.Body, nil
		}

		// 是否重试
		if !conn.reused {
			return nil, err
		}
	}
}

func (tp *Transport) removeConn(target *PersistConn) {
	tp.connPool.Remove(target)
}
