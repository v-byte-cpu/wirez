package connect

import (
	"context"
	"fmt"
	"net"

	"github.com/ginuerzh/gosocks5"
	"github.com/ginuerzh/gosocks5/server"
	"github.com/rs/zerolog"
	"go.uber.org/multierr"
)

func NewSOCKS5ServerHandler(log *zerolog.Logger, connector Connector, transporter Transporter) server.Handler {
	return &serverHandler{log: log, selector: server.DefaultSelector, connector: connector, transporter: transporter}
}

type serverHandler struct {
	log         *zerolog.Logger
	selector    gosocks5.Selector
	connector   Connector
	transporter Transporter
}

func (h *serverHandler) Handle(conn net.Conn) (err error) {
	defer func() {
		if err != nil {
			h.log.Error().Err(err).Msg("")
		}
	}()
	conn = gosocks5.ServerConn(conn, h.selector)
	defer conn.Close()
	req, err := gosocks5.ReadRequest(conn)
	if err != nil {
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
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	cc, err := h.connector.DialContext(ctx, "tcp", req.Addr.String())
	if err != nil {
		return multierr.Append(err, gosocks5.NewReply(gosocks5.HostUnreachable, nil).Write(conn))
	}
	defer cc.Close()

	rep := gosocks5.NewReply(gosocks5.Succeeded, nil)
	if err := rep.Write(conn); err != nil {
		return err
	}

	return h.transporter.Transport(conn, cc)
}
