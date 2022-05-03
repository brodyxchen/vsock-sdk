package vsock

const (
	HeaderSize = 8 // 8个Byte
)

//Header 一排32位
type Header struct {
	Magic   uint16 // 2个byte
	Version uint16

	Action uint16
	Length uint16 //64k

	//Address int16
	//Port int16
}

type Request struct {
	Header
	Body []byte
}

type Response struct {
	Header
	Body []byte
}

type WriteRequest struct {
	Action uint16
	Req    *Request
	Reply  chan error
}
type RequestWrapper struct {
	Req   *Request
	Reply chan *ReadResponse

	callerGone <-chan struct{} // 没有调用者了
}
type ReadResponse struct {
	Rsp *Response
	Err error
}
