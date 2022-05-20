package vsock

import (
	"github.com/brodyxchen/vsock/client"
	"github.com/brodyxchen/vsock/statistics"
)

func NewClient(cfg *client.Config) *client.Client {
	statistics.InitClient()
	statistics.RunClient()

	cli := &client.Client{
		Timeout: cfg.GetTimeout(),
	}
	cli.Init(cfg)

	return cli
}
