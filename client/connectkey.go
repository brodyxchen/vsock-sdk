package client

import (
	"cryptobroker/vsock-sdk/models"
	"strconv"
)

type connectKey struct {
	Uri  string
	Port uint32
}

func (ck *connectKey) From(addr models.Addr) {
	switch ad := addr.(type) {
	case *models.VSockAddr:
		ck.Uri = strconv.FormatUint(uint64(ad.ContextId), 10)
		ck.Port = ad.Port
	case *models.HttpAddr:
		ck.Uri = ad.IP
		ck.Port = ad.Port
	}
}

func (ck *connectKey) Equal(target *connectKey) bool {
	return ck.Uri == target.Uri && ck.Port == target.Port
}
