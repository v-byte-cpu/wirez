package connect

import (
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

var (
	trPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 1<<16)
		},
	}
)

type TimeoutConn struct {
	net.Conn
	// specifies max amount of time to wait for Read/Write calls to complete
	IOTimeout time.Duration
}

func NewTimeoutConn(conn net.Conn, ioTimeout time.Duration) *TimeoutConn {
	return &TimeoutConn{Conn: conn, IOTimeout: ioTimeout}
}

func (c *TimeoutConn) Read(b []byte) (n int, err error) {
	if err = c.SetDeadline(time.Now().Add(c.IOTimeout)); err != nil {
		return
	}
	return c.Conn.Read(b)
}

func (c *TimeoutConn) Write(b []byte) (n int, err error) {
	if err = c.SetDeadline(time.Now().Add(c.IOTimeout)); err != nil {
		return
	}
	return c.Conn.Write(b)
}

type Transporter interface {
	Transport(rw1, rw2 io.ReadWriter) error
}

func NewTransporter(log *zerolog.Logger) Transporter {
	return &transporter{log}
}

type transporter struct {
	log *zerolog.Logger
}

func (t *transporter) Transport(rw1, rw2 io.ReadWriter) error {
	errc := make(chan error, 1)
	copyBuf := func(w io.Writer, r io.Reader) {
		buf := trPool.Get().([]byte)
		defer trPool.Put(buf) //nolint:staticcheck

		_, err := io.CopyBuffer(w, r, buf)
		errc <- err
	}
	go copyBuf(rw1, rw2)
	go copyBuf(rw2, rw1)

	err := <-errc
	t.log.Debug().Err(err).Msg("close connection")
	var terr timeoutError
	if err == io.EOF || (errors.As(err, &terr) && terr.Timeout()) {
		err = nil
	}
	return err
}

type timeoutError interface {
	error
	Timeout() bool
}
