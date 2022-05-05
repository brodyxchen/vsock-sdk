package vsock

type ConnState int

var (
	ConnStateBusy ConnState = 0
	ConnStateIdle ConnState = 1
)
