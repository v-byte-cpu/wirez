package main

import (
	"context"
	"errors"
	"net"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/ginuerzh/gosocks5"
	"github.com/ginuerzh/gosocks5/client"
	"go.uber.org/multierr"
)

const defaultTimeout = 3 * time.Second

// Connector is responsible for connecting to the destination address.
type Connector interface {
	ConnectContext(ctx context.Context, network, address string) (net.Conn, error)
}

type directConnector struct{}

func (*directConnector) ConnectContext(ctx context.Context, network, address string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, network, address)
}

type SocksAddr struct {
	Address string
	Auth    *url.Userinfo
}

func newSOCKS5Connector(connector Connector, socksAddr *SocksAddr) Connector {
	selector := client.DefaultSelector
	if socksAddr.Auth != nil {
		selector = client.NewClientSelector(socksAddr.Auth, gosocks5.MethodUserPass, gosocks5.MethodNoAuth)
	}
	return &socks5Connector{
		connector:    connector,
		selector:     selector,
		socksAddress: socksAddr.Address,
	}
}

type socks5Connector struct {
	connector    Connector
	selector     gosocks5.Selector
	socksAddress string
}

func (c *socks5Connector) ConnectContext(ctx context.Context, _, address string) (conn net.Conn, err error) {
	dstAddr, err := gosocks5.NewAddr(address)
	if err != nil {
		return
	}

	if conn, err = c.connector.ConnectContext(ctx, "tcp", c.socksAddress); err != nil {
		return
	}
	if err = conn.SetDeadline(time.Now().Add(defaultTimeout)); err != nil {
		return
	}
	defer func() {
		err = multierr.Append(err, conn.SetDeadline(time.Time{}))
	}()

	cc := gosocks5.ClientConn(conn, c.selector)
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
		return nil, errors.New("service unavailable")
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
