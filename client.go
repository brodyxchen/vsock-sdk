package vsock

import (
	"github.com/brodyxchen/vsock/client"
	"time"
)

func NewClient(timeout time.Duration) *client.Client {
	cli := &client.Client{
		Timeout: timeout,
	}
	cli.Init()
	return cli
}
