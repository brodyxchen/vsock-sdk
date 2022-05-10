package client

import (
	"context"
	"github.com/brodyxchen/vsock/models"
	"github.com/brodyxchen/vsock/protocols"
	"google.golang.org/protobuf/proto"
	"strconv"
	"sync"
	"time"
)

type Client struct {
	transport *Transport
	Timeout   time.Duration
}

func (cli *Client) Init(cfg *Config) {
	if cli.transport == nil {
		cli.transport = &Transport{
			Name: "transport-" + strconv.FormatInt(time.Now().UnixNano(), 10),
			connPool: ConnPool{
				pool:              make(map[connectKey][]*PersistConn, 0),
				mutex:             sync.RWMutex{},
				idleTimeout:       cfg.GetPoolIdleTimeout(),
				maxCapacityPerKey: cfg.GetPoolMaxCapacity(),
			},
			WriteBufferSize: cfg.GetWriteBufferSize(),
			ReadBufferSize:  cfg.GetReadBufferSize(),
			connIndex:       0,
		}
	}
}

func (cli *Client) Do(addr models.Addr, path string, req []byte) ([]byte, error) {
	return cli.send(addr, path, req, cli.deadline())
}

func (cli *Client) send(addr models.Addr, path string, body []byte, deadline time.Time) ([]byte, error) {
	ctx := context.Background()
	if !deadline.IsZero() {
		ctxDeadline, cancel := context.WithDeadline(ctx, deadline)
		defer cancel()
		ctx = ctxDeadline
	}

	pbReq := &protocols.Request{
		Path: path,
		Req:  body,
	}
	bodyBytes, _ := proto.Marshal(pbReq)

	req := &models.Request{
		Ctx:  ctx,
		Addr: addr,
		Body: bodyBytes,
	}

	rsp, err := cli.transport.roundTrip(req)

	// 系统错误
	if err != nil {
		return nil, err
	}

	// 业务错误
	if rsp.Err != nil {
		return nil, rsp.Err
	}

	return rsp.Body, nil
}

func (cli *Client) SendTest(addr models.Addr, path string, body []byte, deadline time.Time) (int64, []byte, error) {
	ctx := context.Background()
	if !deadline.IsZero() {
		ctxDeadline, cancel := context.WithDeadline(ctx, deadline)
		defer cancel()
		ctx = ctxDeadline
	}

	pbReq := &protocols.Request{
		Path: path,
		Req:  body,
	}
	bodyBytes, _ := proto.Marshal(pbReq)

	req := &models.Request{
		Ctx:  ctx,
		Addr: addr,
		Body: bodyBytes,
	}

	rsp, err := cli.transport.roundTrip(req)
	if err != nil {
		return -1, nil, err
	}

	return rsp.ConnName, rsp.Body, err
}

func (cli *Client) deadline() time.Time {
	if cli.Timeout > 0 {
		return time.Now().Add(cli.Timeout)
	}
	return time.Time{}
}
