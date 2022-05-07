package models

import "strconv"

type Addr interface {
	GetAddr() string
}

type VSockAddr struct {
	ContextId uint32
	Port      uint32
}

func (va *VSockAddr) GetAddr() string {
	return strconv.FormatUint(uint64(va.ContextId), 10) + ":" + strconv.FormatUint(uint64(va.Port), 10)
}

type HttpAddr struct {
	IP   string
	Port uint32
}

func (ha *HttpAddr) GetAddr() string {
	return ha.IP + ":" + strconv.FormatUint(uint64(ha.Port), 10)
}
