package vsock

type Request struct {
	RemoteAddr Addr `json:"-"`

	Action string `json:"action"`
	Data   string `json:"data"`
}

type Response struct {
	Request *Request `json:"-"`

	Result string `json:"result"`
	Data   string `json:"data"`
}

type WriteRequest struct {
	Request *Request
	Reply   chan error
}
type RequestWrapper struct {
	Request *Request
	Reply   chan *ReadResponse

	callerGone <-chan struct{} // 没有调用者了
}
type ReadResponse struct {
	Response *Response
	Err      error
}
