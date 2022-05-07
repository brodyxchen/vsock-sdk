package vsock

import (
	"github.com/brodyxchen/vsock/client"
)

func NewClient(cfg *client.Config) *client.Client {
	cli := &client.Client{
		Timeout: cfg.GetTimeout(),
	}
	cli.Init(cfg)

	return cli
}
