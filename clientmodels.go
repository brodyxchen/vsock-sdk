package vsock

import "context"

type Request struct {
	ctx context.Context

	Addr Addr

	Action uint16
	Body   []byte
}

func (r *Request) Context() context.Context {
	if r.ctx != nil {
		return r.ctx
	}
	return context.Background()
}
