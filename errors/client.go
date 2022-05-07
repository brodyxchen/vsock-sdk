package errors

import "errors"

var (
	ErrUnknownServerErr = errors.New("unknown server error")
	ErrReadTimeout      = errors.New("read response timeout")
	ErrConnEarlyClose   = errors.New("conn early close")

	ErrCtxDone      = errors.New("ctx done")
	ErrCtxRoundDone = errors.New("ctx round done")
	ErrCtxReadDone  = errors.New("ctx read done")
	ErrCtxWriteDone = errors.New("ctx write done")
)
