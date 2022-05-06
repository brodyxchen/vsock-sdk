package server

import (
	"bufio"
	"io"
	"sync"
)

var (
	bufReaderPool sync.Pool
	bufWriterPool sync.Pool
)

func getBufReader(r io.Reader) *bufio.Reader {
	if v := bufReaderPool.Get(); v != nil {
		br := v.(*bufio.Reader)
		br.Reset(r)
		return br
	}
	// Note: if this reader size is ever changed, update
	// TestHandlerBodyClose's assumptions.
	return bufio.NewReader(r)
}

func putBufReader(br *bufio.Reader) {
	br.Reset(nil)
	bufReaderPool.Put(br)
}

func getBufWriter(w io.Writer) *bufio.Writer {
	if v := bufWriterPool.Get(); v != nil {
		bw := v.(*bufio.Writer)
		bw.Reset(w)
		return bw
	}
	return bufio.NewWriter(w)
}

func putBufWriter(bw *bufio.Writer) {
	bw.Reset(nil)
	bufWriterPool.Put(bw)
}
