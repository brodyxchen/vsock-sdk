package client

import (
	"context"
	"github.com/brodyxchen/vsock-sdk/models"
	"github.com/brodyxchen/vsock-sdk/protocols"
	"github.com/brodyxchen/vsock-sdk/statistics"
	"github.com/brodyxchen/vsock-sdk/statistics/metrics"
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

	connGetHist := metrics.NewHistogram(metrics.NewUniformSample(1028))
	connNewHist := metrics.NewHistogram(metrics.NewUniformSample(1028))
	tripHist := metrics.NewHistogram(metrics.NewUniformSample(1028))
	_ = statistics.ClientReg.Register("tp.connGet", connGetHist)
	_ = statistics.ClientReg.Register("tp.connNew", connNewHist)
	_ = statistics.ClientReg.Register("tp.trip", tripHist)
	cli.transport.connGetHist = connGetHist
	cli.transport.connNewHist = connNewHist
	cli.transport.tripHist = tripHist

	sendHist := metrics.NewHistogram(metrics.NewUniformSample(1028))
	sendDoneHist := metrics.NewHistogram(metrics.NewUniformSample(1028))
	receiveHist := metrics.NewHistogram(metrics.NewUniformSample(1028))
	receiveTimeoutHist := metrics.NewHistogram(metrics.NewUniformSample(1028))
	_ = statistics.ClientReg.Register("pconn.send", sendHist)
	_ = statistics.ClientReg.Register("pconn.sendDone", sendDoneHist)
	_ = statistics.ClientReg.Register("pconn.receive", receiveHist)
	_ = statistics.ClientReg.Register("pconn.recvTimeout", receiveTimeoutHist)

	cli.transport.sendHist = sendHist
	cli.transport.sendDoneHist = sendDoneHist
	cli.transport.receiveHist = receiveHist
	cli.transport.receiveTimeoutHist = receiveTimeoutHist
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

func (cli *Client) DialTest(addr models.Addr) (*PersistConn, error) {
	conn, err := cli.transport.DialTest(addr)
	return conn, err
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
	// 系统错误
	if err != nil {
		return -1, nil, err
	}

	// 业务错误
	if rsp.Err != nil {
		return -2, nil, rsp.Err
	}

	return rsp.ConnName, rsp.Body, err
}

func (cli *Client) deadline() time.Time {
	if cli.Timeout > 0 {
		return time.Now().Add(cli.Timeout)
	}
	return time.Time{}
}
