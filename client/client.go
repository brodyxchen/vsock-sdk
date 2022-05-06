package client

import (
	"context"
	"github.com/brodyxchen/vsock/constant"
	"github.com/brodyxchen/vsock/models"
	"strconv"
	"sync"
	"time"
)

type Client struct {
	transport *Transport
	Timeout   time.Duration
}

func (cli *Client) Init() {
	if cli.transport == nil {
		cli.transport = &Transport{
			Name: "transport-" + strconv.FormatInt(time.Now().UnixNano(), 10),
			connPool: ConnPool{
				pool:           make(map[connectKey][]*PersistConn, 0),
				mutex:          sync.RWMutex{},
				idleTimeout:    constant.MaxConnPoolIdleTimeout,
				maxCountPerKey: constant.MaxConnPoolCountPerKey,
			},
			WriteBufferSize: constant.MaxWriteBufferSize,
			ReadBufferSize:  constant.MaxReadBufferSize,
			connIndex:       0,
		}
	}
}

func (cli *Client) Do(addr models.Addr, action uint16, req []byte) ([]byte, error) {
	return cli.send(addr, action, req, cli.deadline())
}

func (cli *Client) send(addr models.Addr, action uint16, body []byte, deadline time.Time) ([]byte, error) {
	ctx := context.Background()
	if !deadline.IsZero() {
		ctxDeadline, cancel := context.WithDeadline(ctx, deadline)
		defer cancel()
		ctx = ctxDeadline
	}

	req := &Request{
		ctx:    ctx,
		Addr:   addr,
		Action: action,
		Body:   body,
	}

	rsp, err := cli.transport.roundTrip(req)

	return rsp.Body, err
}

func (cli *Client) SendTest(addr models.Addr, action uint16, body []byte, deadline time.Time) (int64, []byte, error) {
	ctx := context.Background()
	if !deadline.IsZero() {
		ctxDeadline, cancel := context.WithDeadline(ctx, deadline)
		defer cancel()
		ctx = ctxDeadline
	}

	req := &Request{
		ctx:    ctx,
		Addr:   addr,
		Action: action,
		Body:   body,
	}

	rsp, err := cli.transport.roundTrip(req)

	return rsp.ConnName, rsp.Body, err
}

func (cli *Client) deadline() time.Time {
	if cli.Timeout > 0 {
		return time.Now().Add(cli.Timeout)
	}
	return time.Time{}
}
