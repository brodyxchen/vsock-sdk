package errors

import "errors"

var (
	ErrExceedBody         = errors.New("exceed body size")
	ErrInvalidHeader      = errors.New("invalid header")
	ErrInvalidHeaderMagic = errors.New("invalid header magic number")
	ErrInvalidBody        = errors.New("invalid body")
)
