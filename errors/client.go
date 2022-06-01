package errors

import "errors"

//type Error struct {
//
//}
//
//func (er *Error) Temporary() {
//
//}
//func (er *Error) IsClient() bool {
//
//}
//func (er *Error) IsNetwork() bool {
//
//}
//func (er *Error) IsServer() bool {
//
//}

var (
	ErrUnknownServerErr = errors.New("unknown server error")

	ErrCtxDone      = errors.New("context deadline done")
	ErrCtxReadDone  = errors.New("context read done")
	ErrCtxWriteDone = errors.New("context write done")

	ErrConnIdleTimeout     = errors.New("conn idle timeout")
	ErrOutOfConnectionPool = errors.New("out of connection pool")

	ErrSendErr    = errors.New("client send data err")
	ErrReceiveErr = errors.New("client receive data err")
	ErrClosed     = errors.New("client conn is closed")

	ErrWriteSocketErr = errors.New("write socket err")
	ErrReadSocketErr  = errors.New("read socket err")

	ErrPeekWritingErr = errors.New("peek waiting data err")

	ErrTransportTripClose = errors.New("transport round trip close")
)
