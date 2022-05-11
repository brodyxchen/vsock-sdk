package errors

import "errors"

type Status struct {
	code    uint16
	message string
}

func (st *Status) Error() string {
	return st.message
}

func (st *Status) Code() uint16 {
	return st.code
}

var (
	ErrExceedBody         = errors.New("exceed body size")
	ErrInvalidHeader      = errors.New("invalid header")
	ErrInvalidHeaderMagic = errors.New("invalid header magic number")
	ErrInvalidBody        = errors.New("invalid body")

	ErrNoKeepAlive = errors.New("no keep alive")
)

var (
	StatusInvalidRequest *Status = &Status{401, "invalid request"}
	StatusInvalidPath    *Status = &Status{402, "invalid path"}
)
