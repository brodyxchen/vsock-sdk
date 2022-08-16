package vsock_sdk

import "github.com/brodyxchen/vsock-sdk/statistics"

func SetMetricsEnable(enableClient, enableServer bool) {
	statistics.EnableClient = enableClient
	statistics.EnableServer = enableServer
}
