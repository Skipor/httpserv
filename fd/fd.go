package httpserv

import (
	sys "golang.org/x/sys/unix"
	"io"
)

type Fd int

func (s Fd) Read(p []byte) (n int, err error) {
	n, err = sys.Read(int(s), p)
	if n == 0 && err == nil {
		err = io.EOF
	}
	return
}
func (s Fd) Write(p []byte) (n int, err error) {
	return sys.Write(int(s), p)
}

func (s Fd) Close() error {
	return sys.Close(int(s))
}

