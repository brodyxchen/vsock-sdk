package vsock

import "context"

const (
	HeaderSize = 8 // 8个Byte
)

//Header 一排32位
type sockHeader struct {
	Magic   uint16 // 2个byte
	Version uint16

	Code   uint16 // action or status_code
	Length uint16 //64k
}

type sockRequest struct {
	ctx context.Context
	sockHeader
	Body []byte
}

type sockResponse struct {
	sockHeader
	Body []byte
}

type WriteRequest struct {
	Action uint16
	Req    *sockRequest
	Reply  chan error
}
type RequestWrapper struct {
	Req   *sockRequest
	Reply chan *ReadResponse

	callerGone <-chan struct{} // 没有调用者了
}
type ReadResponse struct {
	Rsp *sockResponse
	Err error
}
