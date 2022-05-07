package models

import "context"

const (
	HeaderSize = 8 // 8个Byte
)

//Header 一排32位
type Header struct {
	Magic   uint16 // 2个byte
	Version uint16

	Code   uint16 // action or status_code
	Length uint16 //64k
}

type Request struct {
	Ctx context.Context
	Header
	Body []byte
}

type Response struct {
	Header
	Body []byte
}

type SendRequest struct {
	Action uint16
	Req    *Request
	Reply  chan error
}
type NotifyReceive struct {
	Req   *Request
	Reply chan *ReceiveResponse

	CallerGone <-chan struct{} // 没有调用者了
}
type ReceiveResponse struct {
	Rsp *Response
	Err error
}
