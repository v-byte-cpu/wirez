package connect

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/ginuerzh/gosocks5"
	"github.com/ginuerzh/gosocks5/server"
	"github.com/rs/zerolog"
	"go.uber.org/multierr"
)

func NewSOCKS5ServerHandler(log *zerolog.Logger, socksTCPConn Connector, socksUDPConn Connector, transporter Transporter) server.Handler {
	return &serverHandler{
		log: log, selector: server.DefaultSelector,
		socksTCPConn: socksTCPConn, socksUDPConn: socksUDPConn, transporter: transporter,
		tcpIOTimeout:   tcpIOTimeout,
		udpIOTimeout:   udpIOTimeout,
		connectTimeout: connectTimeout,
	}
}

type serverHandler struct {
	log            *zerolog.Logger
	selector       gosocks5.Selector
	socksTCPConn   Connector
	socksUDPConn   Connector
	transporter    Transporter
	tcpIOTimeout   time.Duration
	udpIOTimeout   time.Duration
	connectTimeout time.Duration
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
	case gosocks5.CmdUdp:
		return h.handleUDPAssociate(conn, req)
	default:
		return fmt.Errorf("%d: unsupported command", gosocks5.CmdUnsupported)
	}
}

func (h *serverHandler) handleConnect(localConn net.Conn, req *gosocks5.Request) error {
	ctx, cancel := context.WithTimeout(context.Background(), h.connectTimeout)
	defer cancel()
	dstConn, err := h.socksTCPConn.DialContext(ctx, "tcp", req.Addr.String())
	if err != nil {
		return multierr.Append(err, gosocks5.NewReply(gosocks5.HostUnreachable, nil).Write(localConn))
	}
	defer dstConn.Close()

	rep := gosocks5.NewReply(gosocks5.Succeeded, nil)
	if err := rep.Write(localConn); err != nil {
		return err
	}

	localConn = NewTimeoutConn(localConn, h.tcpIOTimeout)
	dstConn = NewTimeoutConn(dstConn, h.tcpIOTimeout)
	return h.transporter.Transport(localConn, dstConn)
}

func (h *serverHandler) handleUDPAssociate(localConn net.Conn, req *gosocks5.Request) error {

	localHost, _, err := net.SplitHostPort(localConn.LocalAddr().String())
	if err != nil {
		return err
	}
	listenAddr, err := net.ResolveUDPAddr("udp", localHost+":")
	if err != nil {
		return err
	}
	listenConn, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		return err
	}
	defer listenConn.Close()
	socksListenAddr, err := gosocks5.NewAddr(listenConn.LocalAddr().String())
	if err != nil {
		return err
	}
	rep := gosocks5.NewReply(gosocks5.Succeeded, socksListenAddr)
	if err := rep.Write(localConn); err != nil {
		return err
	}

	buf := trPool.Get().([]byte)
	n, sourceAddr, err := listenConn.ReadFromUDP(buf)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), h.connectTimeout)
	defer cancel()
	dstAddr := net.IPv4zero
	if req.Addr.Type == gosocks5.AddrIPv6 {
		dstAddr = net.IPv6zero
	}
	dstConn, err := h.socksUDPConn.DialContext(ctx, "udp", dstAddr.String()+":0")
	if err != nil {
		return err
	}
	dstConn = NewTimeoutConn(dstConn, h.udpIOTimeout)
	if _, err = dstConn.Write(buf[:n]); err != nil {
		return err
	}
	trPool.Put(buf) //nolint:staticcheck

	localUDPConn := &firstConnectUDPConn{UDPConn: listenConn, targetAddr: sourceAddr}
	localConn = NewTimeoutConn(localConn, h.udpIOTimeout)
	return h.transporter.Transport(localUDPConn, dstConn)
}

type firstConnectUDPConn struct {
	*net.UDPConn
	targetAddr *net.UDPAddr
}

func (c *firstConnectUDPConn) Read(b []byte) (n int, err error) {
	n, addr, err := c.ReadFromUDP(b)
	if err != nil {
		return
	}
	if !addr.IP.Equal(c.targetAddr.IP) || addr.Port != c.targetAddr.Port {
		return 0, errors.New("source ip address is invalid")
	}
	return
}

func (c *firstConnectUDPConn) Write(b []byte) (n int, err error) {
	return c.WriteToUDP(b, c.targetAddr)
}
