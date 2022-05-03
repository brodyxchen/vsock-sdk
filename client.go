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

func (cli *Client) Do(ad Addr, action uint16, req []byte) ([]byte, error) {
	return cli.send(ad, action, req, cli.deadline())
}

func (cli *Client) send(ad Addr, action uint16, req []byte, deadline time.Time) ([]byte, error) {
	if cli.transport == nil {
		cli.transport = &Transport{
			connPool:        make(map[connectKey][]*PersistConn, 0),
			idleTimeout:     time.Minute,
			maxIdleCount:    2,
			WriteBufferSize: 1024 * 4,
			ReadBufferSize:  1024 * 4,
		}
	}
	rsp, err := cli.transport.roundTrip(ad, action, req)

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
