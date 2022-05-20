package vsock

import (
	"cryptobroker/vsock-sdk/client"
	"cryptobroker/vsock-sdk/statistics"
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
