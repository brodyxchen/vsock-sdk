package client

import (
	"context"
	"github.com/brodyxchen/vsock/models"
)

type Request struct {
	ctx context.Context

	Addr models.Addr

	Action uint16
	Body   []byte
}

func (r *Request) Context() context.Context {
	if r.ctx != nil {
		return r.ctx
	}
	return context.Background()
}

type Response struct {
	Req *Request

	ConnName int64

	Code uint16
	Body []byte
}
