package errors

import "errors"

var (
	ErrUnknownServerErr = errors.New("unknown server error")
	ErrReadTimeout      = errors.New("read response timeout")
	ErrConnEarlyClose   = errors.New("conn early close")
	ErrCtxDone          = errors.New("ctx done")
)
