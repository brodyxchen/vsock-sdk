package vsock

import "strconv"

type connectKey struct {
	Uri string
	Port uint32
}

func (ck *connectKey) From(addr Addr) {
	switch ad := addr.(type) {
	case *VSockAddr:
		ck.Uri = strconv.FormatUint(uint64(ad.ContextId), 10)
		ck.Port = ad.Port
	case *HttpAddr:
		ck.Uri = ad.IP
		ck.Port = ad.Port
	}
}

func (ck *connectKey) Equal(target *connectKey) bool {
	return ck.Uri == target.Uri && ck.Port == target.Port
}

