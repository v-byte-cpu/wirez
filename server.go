package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/ginuerzh/gosocks5"
	"github.com/ginuerzh/gosocks5/server"
)

func newSOCKS5ServerHandler(connector Connector) server.Handler {
	return &serverHandler{server.DefaultSelector, connector}
}

type serverHandler struct {
	selector  gosocks5.Selector
	connector Connector
}

func (h *serverHandler) Handle(conn net.Conn) error {
	conn = gosocks5.ServerConn(conn, h.selector)
	defer conn.Close()
	req, err := gosocks5.ReadRequest(conn)
	if err != nil {
		log.Println("error", err)
		return err
	}

	switch req.Cmd {
	case gosocks5.CmdConnect:
		return h.handleConnect(conn, req)
	default:
		return fmt.Errorf("%d: unsupported command", gosocks5.CmdUnsupported)
	}
}

func (h *serverHandler) handleConnect(conn net.Conn, req *gosocks5.Request) error {
	// TODO configure timeout
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	cc, err := h.connector.ConnectContext(ctx, "tcp", req.Addr.String())
	if err != nil {
		log.Println("error connect", err)
		rep := gosocks5.NewReply(gosocks5.HostUnreachable, nil)
		rep.Write(conn)
		return err
	}
	defer cc.Close()

	rep := gosocks5.NewReply(gosocks5.Succeeded, nil)
	if err := rep.Write(conn); err != nil {
		log.Println("error", err)
		return err
	}

	return transport(conn, cc)
}

var (
	trPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 2048)
		},
	}
)

func transport(rw1, rw2 io.ReadWriter) error {
	errc := make(chan error, 1)
	// TODO read/write timeouts
	copyBuf := func(w io.Writer, r io.Reader) {
		buf := trPool.Get().([]byte)
		defer trPool.Put(buf)

		_, err := io.CopyBuffer(w, r, buf)
		errc <- err
	}
	go copyBuf(rw1, rw2)
	go copyBuf(rw2, rw1)

	err := <-errc
	if err == io.EOF {
		err = nil
	}
	return err
}
