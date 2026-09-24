//go:build linux

package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/v-byte-cpu/wirez/pkg/connect"
	"go.uber.org/multierr"
	"golang.org/x/sys/unix"
)

type renamedTypeFlagValue struct {
	pflag.Value
	name        string
	hideDefault bool
}

func (v *renamedTypeFlagValue) Type() string {
	return v.name
}

func (v *renamedTypeFlagValue) String() string {
	if v.hideDefault {
		return ""
	}
	return v.Value.String()
}

func setLogLevel(log *zerolog.Logger, verboseLevel int) *zerolog.Logger {
	level := zerolog.InfoLevel
	switch {
	case verboseLevel < 0:
		level = zerolog.Disabled
	case verboseLevel == 1:
		level = zerolog.DebugLevel
	case verboseLevel >= 2:
		level = zerolog.TraceLevel
	}
	result := log.Level(level)
	return &result
}

func newRunCmd(log *zerolog.Logger) *runCmd {
	c := &runCmd{}

	cmd := &cobra.Command{
		Use: "run [flags] command",
		Example: strings.Join([]string{
			"wirez run -F 127.0.0.1:1234 bash",
			"wirez run -F 127.0.0.1:1234 --dns 1.1.1.1 -- curl example.com",
			"wirez run -F 127.0.0.1:1234 -L 53:1.1.1.1:53/udp -- curl example.com"}, "\n"),
		Short: "Proxy application traffic through the socks5 server",
		Long:  "Run a command in an unprivileged container that transparently proxies application traffic through the socks5 server",
		RunE: func(_ *cobra.Command, args []string) error {
			return c.run(log, args)
		},
	}

	c.opts.initCliFlags(cmd)

	c.cmd = cmd
	return c
}

func (c *runCmd) run(log *zerolog.Logger, args []string) error {
	if err := c.validate(); err != nil {
		return err
	}
	log = setLogLevel(log, c.opts.VerboseLevel)
	log.Debug().Strs("forward", c.opts.ForwardProxies).Msg("")
	log.Debug().Strs("local_address_mappings", c.opts.LocalAddressMappings).Msg("")
	log.Debug().Strs("dns_servers", c.opts.DNSServers).Msg("")
	log.Debug().Str("dns_force_server", c.opts.DNSForceServer).Msg("")
	forwardProxies, err := parseProxyURLs(c.opts.ForwardProxies)
	if err != nil {
		return err
	}
	dnsServers, err := parseDNSServers(c.opts.DNSServers)
	if err != nil {
		return err
	}
	dnsForceServer, err := parseDNSForceServer(c.opts.DNSForceServer)
	if err != nil {
		return err
	}
	// forced DNS mappings come before user mappings so an explicit -L wins
	addressMappings := append([]string{}, dnsForceMappings(dnsForceServer)...)
	addressMappings = append(addressMappings, c.opts.LocalAddressMappings...)
	nat, err := parseAddressMapper(addressMappings)
	if err != nil {
		return err
	}
	natDNS, err := parseAddressMapper(dnsServerMappings(dnsServers))
	if err != nil {
		return err
	}

	parentFd, childFd, err := newUnixSocketPair()
	if err != nil {
		return err
	}
	defer unix.Close(parentFd)
	defer unix.Close(childFd)

	proc := c.newProcess(childFd, dnsServerAddresses(dnsServers), dnsUseVC(dnsServers), args)
	if err = proc.Start(); err != nil {
		return err
	}
	parentConn := newParentUnixSocketConn(parentFd)
	tunFd, err := parentConn.ReceiveFd()
	if err != nil {
		return err
	}
	log.Debug().Int("fd", tunFd).Msg("got tun device")
	defer unix.Close(tunFd)
	tunMTU, err := parentConn.ReceiveMTU()
	if err != nil {
		return err
	}
	log.Debug().Uint32("mtu", tunMTU).Msg("")

	tcpConn, udpConn := c.newConnectors(log, forwardProxies, nat, natDNS)
	stack, err := connect.NewNetworkStack(log, tunFd, tunMTU, tunNetworkAddr,
		tcpConn, udpConn, connect.NewTransporter(log))
	if err != nil {
		return err
	}
	defer stack.Close()
	if err = parentConn.SendACK(); err != nil {
		return err
	}
	return proc.Wait()
}

func (c *runCmd) validate() error {
	if c.opts.ContainerUID < 0 {
		return errors.New("uid is negative")
	}
	if c.opts.ContainerGID < 0 {
		return errors.New("gid is negative")
	}
	if len(c.opts.ForwardProxies) == 0 {
		return errors.New("forward proxies list is empty")
	}
	return nil
}

func (c *runCmd) newProcess(childFd int, dnsServers []string, dnsUseVC bool, args []string) *exec.Cmd {
	privileged := os.Geteuid() == 0
	procArgs := []string{"runc", "--unix-fd", strconv.Itoa(childFd),
		fmt.Sprintf("--privileged=%t", privileged), "--uid", strconv.Itoa(c.opts.ContainerUID),
		"--gid", strconv.Itoa(c.opts.ContainerGID)}
	for _, dnsServer := range dnsServers {
		procArgs = append(procArgs, "--dns", dnsServer)
	}
	if dnsUseVC {
		procArgs = append(procArgs, "--dns-use-vc")
	}
	procArgs = append(procArgs, "--")
	// Arguments after -- are intentionally supplied by the CLI user.
	procArgs = append(procArgs, args...)
	proc := exec.Command("/proc/self/exe", procArgs...) //nolint:gosec
	proc.Stdin = os.Stdin
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	cloneFlags := uintptr(syscall.CLONE_NEWUTS | syscall.CLONE_NEWNET)
	if len(dnsServers) > 0 {
		// own mount namespace is required to override /etc/resolv.conf
		cloneFlags |= syscall.CLONE_NEWNS
	}
	if privileged {
		proc.SysProcAttr = &syscall.SysProcAttr{Cloneflags: cloneFlags}
		return proc
	}
	proc.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  cloneFlags | syscall.CLONE_NEWUSER,
		Credential:  &syscall.Credential{Uid: 0, Gid: uint32(c.opts.ContainerGID)},
		UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Geteuid(), Size: 1}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: c.opts.ContainerGID, HostID: os.Getegid(), Size: 1}},
	}
	return proc
}

func (c *runCmd) newConnectors(
	log *zerolog.Logger,
	forwardProxies []*connect.SocksAddr,
	nat connect.AddressMapper,
	natDNS connect.AddressMapper,
) (connect.Connector, connect.Connector) {
	directConn := connect.NewDirectConnector()
	tcpConn := directConn
	tcpConns := make([]connect.Connector, 0, len(c.opts.ForwardProxies)+1)
	tcpConns = append(tcpConns, directConn)
	for _, proxyAddr := range forwardProxies {
		tcpConn = connect.NewSOCKS5Connector(tcpConn, proxyAddr)
		tcpConns = append(tcpConns, tcpConn)
	}
	udpConn := directConn
	for i, proxyAddr := range forwardProxies {
		udpConn = connect.NewSOCKS5UDPConnector(log, tcpConns[i], udpConn, proxyAddr)
	}
	// DNS is rewritten to the configured servers and resolved through the proxy
	tcpConn = connect.NewRewritingConnector(tcpConn, natDNS)
	udpConn = connect.NewRewritingConnector(udpConn, natDNS)
	tcpConn = connect.NewLocalForwardingConnector(directConn, tcpConn, nat)
	udpConn = connect.NewLocalForwardingConnector(directConn, udpConn, nat)
	return tcpConn, udpConn
}

type runCmd struct {
	cmd  *cobra.Command
	opts runCmdOpts
}

type runCmdOpts struct {
	ForwardProxies       []string
	LocalAddressMappings []string
	DNSServers           []string
	DNSForceServer       string
	VerboseLevel         int
	ContainerUID         int
	ContainerGID         int
}

func (o *runCmdOpts) initCliFlags(cmd *cobra.Command) {
	cmd.Flags().StringArrayVarP(&o.ForwardProxies, "forward", "F", nil, "set socks5 proxy address to forward TCP/UDP packets")
	forwardFlag := cmd.Flags().Lookup("forward")
	forwardFlag.Value = &renamedTypeFlagValue{Value: forwardFlag.Value, name: "address", hideDefault: true}

	cmd.Flags().CountVarP(&o.VerboseLevel, "verbose", "v", "log verbose level")
	verboseFlag := cmd.Flags().Lookup("verbose")
	verboseFlag.Value = &renamedTypeFlagValue{Value: verboseFlag.Value}

	cmd.Flags().StringArrayVarP(&o.LocalAddressMappings, "local", "L", nil, "specifies that connections to the target host and TCP/UDP port are to be directly forwarded to the given host and port")
	localFlag := cmd.Flags().Lookup("local")
	localFlag.Value = &renamedTypeFlagValue{Value: localFlag.Value, name: "[target_host:]port:host:hostport[/proto]", hideDefault: true}

	cmd.Flags().StringArrayVar(&o.DNSServers, "dns", nil, "set DNS server for the wrapped process in its own /etc/resolv.conf, e.g. 1.1.1.1, 1.1.1.1:5353, 1.1.1.1/udp (can be specified multiple times)")
	dnsFlag := cmd.Flags().Lookup("dns")
	dnsFlag.Value = &renamedTypeFlagValue{Value: dnsFlag.Value, name: "server_address", hideDefault: true}

	cmd.Flags().StringVar(&o.DNSForceServer, "dns-force", "", "intercept all DNS traffic (UDP/TCP port 53) and send it directly to this server, even for programs that ignore /etc/resolv.conf")

	cmd.Flags().IntVar(&o.ContainerUID, "uid", os.Geteuid(), "set uid of container process")
	cmd.Flags().IntVar(&o.ContainerGID, "gid", os.Getegid(), "set gid of container process")
}

func newUnixSocketPair() (parentFd, childFd int, err error) {
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		return
	}
	parentFd = fds[0]
	childFd = fds[1]

	// set clo_exec flag on parent file descriptor
	_, err = unix.FcntlInt(uintptr(parentFd), unix.F_SETFD, unix.FD_CLOEXEC)
	if err != nil {
		err = multierr.Append(err, unix.Close(parentFd))
		err = multierr.Append(err, unix.Close(childFd))
	}
	return
}

type parentUnixSocketConn struct {
	socketFd   int
	socketFile *os.File
}

func newParentUnixSocketConn(socketFd int) *parentUnixSocketConn {
	return &parentUnixSocketConn{
		socketFd:   socketFd,
		socketFile: os.NewFile(uintptr(socketFd), "parentPipe"),
	}
}

func (c *parentUnixSocketConn) Close() error {
	return unix.Close(c.socketFd)
}

func (c *parentUnixSocketConn) ReceiveFd() (fd int, err error) {
	// receive socket control message
	b := make([]byte, unix.CmsgSpace(4))
	if _, _, _, _, err = unix.Recvmsg(c.socketFd, nil, b, 0); err != nil {
		return
	}

	// parse socket control message
	cmsgs, err := unix.ParseSocketControlMessage(b)
	if err != nil {
		return 0, fmt.Errorf("parse socket control message: %w", err)
	}

	tunFds, err := unix.ParseUnixRights(&cmsgs[0])
	if err != nil {
		return 0, err
	}
	if len(tunFds) == 0 {
		return 0, errors.New("tun fds slice is empty")
	}
	return tunFds[0], nil
}

func (c *parentUnixSocketConn) ReceiveMTU() (mtu uint32, err error) {
	var msg MTUMessage
	if err = json.NewDecoder(c.socketFile).Decode(&msg); err != nil {
		return
	}
	return msg.MTU, nil
}

func (c *parentUnixSocketConn) SendACK() error {
	return json.NewEncoder(c.socketFile).Encode(&ACKMessage{ACK: true})
}

type ACKMessage struct {
	ACK bool
}
