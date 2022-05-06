package vsock

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrUnknownServerErr = errors.New("unknown server error")
)

type Client struct {
	transport *Transport
	Timeout   time.Duration
}

func (cli *Client) Do(ad Addr, action uint16, req []byte) ([]byte, error) {
	return cli.send(ad, action, req, cli.deadline())
}

func (cli *Client) send(addr Addr, action uint16, body []byte, deadline time.Time) ([]byte, error) {
	if cli.transport == nil {
		cli.transport = &Transport{
			connPool: ConnPool{
				pool:     make(map[connectKey][]*PersistConn, 0),
				mutex:    sync.RWMutex{},
				maxCount: 2,
			},
			idleTimeout:     time.Minute,
			WriteBufferSize: 1024 * 4,
			ReadBufferSize:  1024 * 4,
			connIndex:       0,
		}
	}

	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	req := &Request{
		ctx:    ctx,
		Addr:   addr,
		Action: action,
		Body:   body,
	}

	rsp, err := cli.transport.roundTrip(req)

	return rsp, err
}

func (cli *Client) sendTest(ad Addr, action uint16, req []byte, deadline time.Time) (int64, []byte, error) {
	if cli.transport == nil {
		cli.transport = &Transport{
			connPool: ConnPool{
				pool:     make(map[connectKey][]*PersistConn, 0),
				mutex:    sync.RWMutex{},
				maxCount: 2,
			},
			idleTimeout:     time.Minute,
			WriteBufferSize: 1024 * 4,
			ReadBufferSize:  1024 * 4,
			connIndex:       0,
		}
	}
	index, rsp, err := cli.transport.roundTripTest(ad, action, req)

	return index, rsp, err
}

func (cli *Client) deadline() time.Time {
	if cli.Timeout > 0 {
		return time.Now().Add(cli.Timeout)
	}
	return time.Time{}
}
