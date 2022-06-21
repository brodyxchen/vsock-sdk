package vsock

import (
	"github.com/brodyxchen/vsock-sdk/client"
	"github.com/brodyxchen/vsock-sdk/statistics"
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
