package connect

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/rs/zerolog"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/fdbased"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

type NetworkStack struct {
	*stack.Stack
	log            *zerolog.Logger
	socksTCPConn   Connector
	socksUDPConn   Connector
	transporter    Transporter
	TcpIOTimeout   time.Duration
	UdpIOTimeout   time.Duration
	ConnectTimeout time.Duration
}

func NewNetworkStack(log *zerolog.Logger, fd int, mtu uint32, tunNetworkAddr string,
	socksTCPConn Connector, socksUDPConn Connector, transporter Transporter) (*NetworkStack, error) {
	s := &NetworkStack{
		log:            log,
		socksTCPConn:   socksTCPConn,
		socksUDPConn:   socksUDPConn,
		TcpIOTimeout:   tcpIOTimeout,
		UdpIOTimeout:   udpIOTimeout,
		ConnectTimeout: connectTimeout,
		transporter:    transporter,
		Stack: stack.New(stack.Options{
			NetworkProtocols: []stack.NetworkProtocolFactory{
				ipv4.NewProtocol,
				ipv6.NewProtocol,
			},
			TransportProtocols: []stack.TransportProtocolFactory{
				tcp.NewProtocol,
				udp.NewProtocol,
			},
			DefaultIPTables: defaultIPTables,
		}),
	}

	ep, err := fdbased.New(&fdbased.Options{
		MTU: mtu,
		FDs: []int{fd},
		// TUN only
		EthernetHeader: false,
	})
	if err != nil {
		return nil, err
	}

	var defaultNICID tcpip.NICID = 0x01
	if err := s.CreateNIC(defaultNICID, ep); err != nil {
		return nil, errors.New(err.String())
	}

	if err := s.SetPromiscuousMode(defaultNICID, true); err != nil {
		return nil, errors.New(err.String())
	}
	if err := s.SetSpoofing(defaultNICID, true); err != nil {
		return nil, errors.New(err.String())
	}
	if err = s.SetupRouting(defaultNICID, tunNetworkAddr); err != nil {
		return nil, err
	}

	s.setTCPHandler()
	s.setUDPHandler()
	return s, nil
}

func (s *NetworkStack) SetupRouting(nic tcpip.NICID, assignNet string) error {
	_, ipNet, err := net.ParseCIDR(assignNet)
	if err != nil {
		return fmt.Errorf("unable to ParseCIDR(%s): %w", assignNet, err)
	}

	subnet, err := tcpip.NewSubnet(tcpip.Address(ipNet.IP), tcpip.AddressMask(ipNet.Mask))
	if err != nil {
		return fmt.Errorf("unable to NewSubnet(%s): %w", ipNet, err)
	}

	rt := s.GetRouteTable()
	rt = append(rt, tcpip.Route{
		Destination: subnet,
		NIC:         nic,
	})
	s.SetRouteTable(rt)
	return nil
}

func (s *NetworkStack) setTCPHandler() {
	tcpForwarder := tcp.NewForwarder(s.Stack, 0, 2<<10, func(r *tcp.ForwarderRequest) {
		var wq waiter.Queue
		id := r.ID()
		s.log.Debug().Str("handler", "tcp").
			Stringer("localAddress", id.LocalAddress).Uint16("localPort", id.LocalPort).
			Stringer("fromAddress", id.RemoteAddress).Uint16("fromPort", id.RemotePort).Msg("received request")
		ep, err := r.CreateEndpoint(&wq)
		if err != nil {
			s.log.Error().Str("handler", "tcp").Stringer("error", err).Msg("")
			// prevent potential half-open TCP connection leak.
			r.Complete(true)
			return
		}
		r.Complete(false)

		go func() {
			if err := s.handleTCP(gonet.NewTCPConn(&wq, ep), &id); err != nil {
				s.log.Error().Str("handler", "tcp").Err(err).Msg("")
			}
		}()
	})
	s.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpForwarder.HandlePacket)
}

func (s *NetworkStack) setUDPHandler() {
	udpForwarder := udp.NewForwarder(s.Stack, func(r *udp.ForwarderRequest) {
		var wq waiter.Queue
		id := r.ID()
		s.log.Debug().Str("handler", "udp").
			Stringer("localAddress", id.LocalAddress).Uint16("localPort", id.LocalPort).
			Stringer("fromAddress", id.RemoteAddress).Uint16("fromPort", id.RemotePort).Msg("received request")
		ep, err := r.CreateEndpoint(&wq)
		if err != nil {
			s.log.Error().Str("handler", "udp").Stringer("error", err).Msg("")
			return
		}
		go func() {
			if err := s.handleUDP(gonet.NewUDPConn(s.Stack, &wq, ep), &id); err != nil {
				s.log.Error().Str("handler", "udp").Err(err).Msg("")
			}
		}()
	})
	s.SetTransportProtocolHandler(udp.ProtocolNumber, udpForwarder.HandlePacket)
}

func (s *NetworkStack) handleTCP(localConn net.Conn, id *stack.TransportEndpointID) (err error) {
	defer localConn.Close()

	address := fmt.Sprintf("%s:%v", id.LocalAddress, id.LocalPort)

	ctx, cancel := context.WithTimeout(context.Background(), s.ConnectTimeout)
	defer cancel()
	dstConn, err := s.socksTCPConn.DialContext(ctx, "tcp", address)
	if err != nil {
		return
	}
	defer dstConn.Close()

	localConn = NewTimeoutConn(localConn, s.TcpIOTimeout)
	dstConn = NewTimeoutConn(dstConn, s.TcpIOTimeout)
	// relay TCP connections
	return s.transporter.Transport(localConn, dstConn)
}

func (s *NetworkStack) handleUDP(localConn net.Conn, id *stack.TransportEndpointID) (err error) {
	defer localConn.Close()

	dstAddress := fmt.Sprintf("%s:%v", id.LocalAddress, id.LocalPort)
	s.log.Debug().Str("dstAddr", dstAddress).Msg("handleUDP called")

	ctx, cancel := context.WithTimeout(context.Background(), s.ConnectTimeout)
	defer cancel()
	dstConn, err := s.socksUDPConn.DialContext(ctx, "udp", dstAddress)
	if err != nil {
		return
	}
	defer dstConn.Close()

	localConn = NewTimeoutConn(localConn, s.UdpIOTimeout)
	dstConn = NewTimeoutConn(dstConn, s.UdpIOTimeout)
	// relay UDP connections
	return s.transporter.Transport(localConn, dstConn)
}

// defaultIPTables creates iptables rules that allow only TCP and UDP traffic
func defaultIPTables(clock tcpip.Clock, rand *rand.Rand) *stack.IPTables {
	const (
		TCPAllowRuleNum = iota
		_
		DropRuleNum
		AllowRuleNum
	)
	iptables := stack.DefaultTables(clock, rand)
	ipv4filter := iptables.GetTable(stack.FilterID, false)
	ipv4filter.Rules = []stack.Rule{
		{
			Filter: stack.IPHeaderFilter{
				Protocol:      header.TCPProtocolNumber,
				CheckProtocol: true,
			},
			Target: &stack.AcceptTarget{NetworkProtocol: header.IPv4ProtocolNumber},
		},
		{
			Filter: stack.IPHeaderFilter{
				Protocol:      header.UDPProtocolNumber,
				CheckProtocol: true,
			},
			Target: &stack.AcceptTarget{NetworkProtocol: header.IPv4ProtocolNumber},
		},
		{Target: &stack.DropTarget{NetworkProtocol: header.IPv4ProtocolNumber}},
		{Target: &stack.AcceptTarget{NetworkProtocol: header.IPv4ProtocolNumber}},
	}
	ipv4filter.BuiltinChains = [stack.NumHooks]int{
		stack.Prerouting:  TCPAllowRuleNum,
		stack.Input:       TCPAllowRuleNum,
		stack.Forward:     TCPAllowRuleNum,
		stack.Output:      TCPAllowRuleNum,
		stack.Postrouting: AllowRuleNum,
	}
	ipv4filter.Underflows = [stack.NumHooks]int{
		stack.Prerouting:  DropRuleNum,
		stack.Input:       DropRuleNum,
		stack.Forward:     DropRuleNum,
		stack.Output:      DropRuleNum,
		stack.Postrouting: DropRuleNum,
	}
	iptables.ReplaceTable(stack.FilterID, ipv4filter, false)

	ipv6filter := iptables.GetTable(stack.FilterID, true)
	ipv6filter.Rules = []stack.Rule{
		{
			Filter: stack.IPHeaderFilter{
				Protocol:      header.TCPProtocolNumber,
				CheckProtocol: true,
			},
			Target: &stack.AcceptTarget{NetworkProtocol: header.IPv6ProtocolNumber},
		},
		{
			Filter: stack.IPHeaderFilter{
				Protocol:      header.UDPProtocolNumber,
				CheckProtocol: true,
			},
			Target: &stack.AcceptTarget{NetworkProtocol: header.IPv6ProtocolNumber},
		},
		{Target: &stack.DropTarget{NetworkProtocol: header.IPv6ProtocolNumber}},
		{Target: &stack.AcceptTarget{NetworkProtocol: header.IPv6ProtocolNumber}},
	}
	ipv6filter.BuiltinChains = [stack.NumHooks]int{
		stack.Prerouting:  TCPAllowRuleNum,
		stack.Input:       TCPAllowRuleNum,
		stack.Forward:     TCPAllowRuleNum,
		stack.Output:      TCPAllowRuleNum,
		stack.Postrouting: AllowRuleNum,
	}
	ipv6filter.Underflows = [stack.NumHooks]int{
		stack.Prerouting:  DropRuleNum,
		stack.Input:       DropRuleNum,
		stack.Forward:     DropRuleNum,
		stack.Output:      DropRuleNum,
		stack.Postrouting: DropRuleNum,
	}
	iptables.ReplaceTable(stack.FilterID, ipv6filter, true)

	return iptables
}
