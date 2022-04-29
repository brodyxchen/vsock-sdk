package vsock

import (
	"bytes"
	"io"
	"net"
	"time"
)

type Client struct {
	transport *Transport
	Timeout   time.Duration
}

func (cli *Client) Do(req *Request) (*Response, error) {
	return cli.send(req, cli.deadline())
}

func (cli *Client) send(req *Request, deadline time.Time) (*Response, error) {
	if cli.transport == nil {
		cli.transport = &Transport{
			connPool:        make(map[connectKey][]*PersistConn, 0),
			idleTimeout:     time.Minute,
			maxIdleCount:    2,
			WriteBufferSize: 1024*4,
			ReadBufferSize:  1024*4,
		}
	}
	rsp, err := cli.transport.roundTrip(req)

	return rsp, err
}

func (cli *Client) deadline() time.Time {
	if cli.Timeout > 0 {
		return time.Now().Add(cli.Timeout)
	}
	return time.Time{}
}

func (cli *Client) ReadDDD(conn net.Conn) (string, error) {
	var dataBuf bytes.Buffer
	buf := make([]byte, 1024*4)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				return dataBuf.String(), nil
			}
			return "", err
		}
		dataBuf.Write(buf[:n])
	}
}
