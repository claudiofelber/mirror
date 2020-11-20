package internal

import (
	"io"
)

type PassThru struct {
	io.Reader
	Callback func(written int64)
	written  int64
}

func (pt *PassThru) Read(p []byte) (int, error) {
	n, err := pt.Reader.Read(p)
	pt.written += int64(n)

	if err == nil && pt.Callback != nil {
		pt.Callback(pt.written)
	}

	return n, err
}
