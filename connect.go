package main

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"time"

	"github.com/ginuerzh/gosocks5"
	"github.com/ginuerzh/gosocks5/client"
)

const defaultTimeout = 3 * time.Second

// Connector is responsible for connecting to the destination address.
type Connector interface {
	ConnectContext(ctx context.Context, network, address string) (net.Conn, error)
}

type directConnector struct{}

func (c *directConnector) ConnectContext(ctx context.Context, network, address string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, network, address)
}

func newSOCKS5Connector(connector Connector, socksAddr string) Connector {
	return &socks5Connector{connector, socksAddr}
}

type socks5Connector struct {
	connector Connector
	socksAddr string
}

func (c *socks5Connector) ConnectContext(ctx context.Context, network, address string) (conn net.Conn, err error) {
	dstAddr, err := gosocks5.NewAddr(address)
	if err != nil {
		return
	}

	if conn, err = c.connector.ConnectContext(ctx, "tcp", c.socksAddr); err != nil {
		return
	}
	conn.SetDeadline(time.Now().Add(defaultTimeout))
	defer conn.SetDeadline(time.Time{})

	cc := gosocks5.ClientConn(conn, client.DefaultSelector)
	if err = cc.Handleshake(); err != nil {
		return
	}
	conn = cc

	req := gosocks5.NewRequest(gosocks5.CmdConnect, dstAddr)
	if err = req.Write(conn); err != nil {
		return
	}
	reply, err := gosocks5.ReadReply(conn)
	if err != nil {
		return
	}
	if reply.Rep != gosocks5.Succeeded {
		return nil, errors.New("Service unavailable")
	}
	return
}

// TODO performance metrics
// TODO dynamic add/remove connectors
// TODO rotation policy (more/less trusted nodes etc.)
func newRotationConnector(connectors []Connector) Connector {
	return &rotationConnector{connectors: connectors}
}

type rotationConnector struct {
	connectors []Connector
	robin      uint32
}

func (c *rotationConnector) ConnectContext(ctx context.Context, network, address string) (conn net.Conn, err error) {
	i := int(atomic.AddUint32(&c.robin, 1) % uint32(len(c.connectors)))
	return c.connectors[i].ConnectContext(ctx, network, address)
}
