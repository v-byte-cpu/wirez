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
	"github.com/v-byte-cpu/wirez/pkg/connect"
	"go.uber.org/multierr"
	"golang.org/x/sys/unix"
)

func newRunCmd(log *zerolog.Logger) *runCmd {
	c := &runCmd{}

	cmd := &cobra.Command{
		Use: "run [flags] command",
		Example: strings.Join([]string{
			"wirez run -F 127.0.0.1:1234 bash",
			"wirez run -F 127.0.0.1:1234 -L 53:1.1.1.1:53/udp -- curl example.com"}, "\n"),
		Short: "Proxy application traffic through the socks5 server",
		Long:  "Run a command in an unprivileged container that transparently proxies application traffic through the socks5 server",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if c.opts.ContainerUID < 0 {
				return errors.New("uid is negative")
			}
			if c.opts.ContainerGID < 0 {
				return errors.New("gid is negative")
			}
			if len(c.opts.ForwardProxies) == 0 {
				return errors.New("forward proxies list is empty")
			}
			log = setLogLevel(log, c.opts.VerboseLevel)
			log.Debug().Strs("forward", c.opts.ForwardProxies).Msg("")
			log.Debug().Strs("local_address_mappings", c.opts.LocalAddressMappings).Msg("")
			forwardProxies, err := parseProxyURLs(c.opts.ForwardProxies)
			if err != nil {
				return
			}

			nat, err := parseAddressMapper(c.opts.LocalAddressMappings)
			if err != nil {
				return
			}

			parentFd, childFd, err := newUnixSocketPair()
			if err != nil {
				return
			}
			defer unix.Close(parentFd)
			defer unix.Close(childFd)

			privileged := os.Geteuid() == 0
			proc := exec.Command("/proc/self/exe", append([]string{"runc",
				"--unix-fd", strconv.Itoa(childFd), fmt.Sprintf("--privileged=%t", privileged),
				"--uid", strconv.Itoa(c.opts.ContainerUID), "--gid", strconv.Itoa(c.opts.ContainerGID), "--"}, args...)...)
			proc.Stdin = os.Stdin
			proc.Stdout = os.Stdout
			proc.Stderr = os.Stderr

			if privileged {
				proc.SysProcAttr = &syscall.SysProcAttr{
					Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWNET,
				}
			} else {
				proc.SysProcAttr = &syscall.SysProcAttr{
					Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWNET | syscall.CLONE_NEWUSER,
					Credential: &syscall.Credential{Uid: 0, Gid: uint32(c.opts.ContainerGID)},
					UidMappings: []syscall.SysProcIDMap{
						{ContainerID: 0, HostID: os.Geteuid(), Size: 1},
					},
					GidMappings: []syscall.SysProcIDMap{
						{ContainerID: c.opts.ContainerGID, HostID: os.Getegid(), Size: 1},
					},
				}
			}
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

			dconn := connect.NewDirectConnector()
			socksTCPConn := dconn
			socksTCPConns := make([]connect.Connector, 0, len(c.opts.ForwardProxies)+1)
			socksTCPConns = append(socksTCPConns, dconn)
			for _, proxyAddr := range forwardProxies {
				socksTCPConn = connect.NewSOCKS5Connector(socksTCPConn, proxyAddr)
				socksTCPConns = append(socksTCPConns, socksTCPConn)
			}
			socksUDPConn := dconn
			for i, proxyAddr := range forwardProxies {
				socksUDPConn = connect.NewSOCKS5UDPConnector(log, socksTCPConns[i], socksUDPConn, proxyAddr)
			}
			socksTCPConn = connect.NewLocalForwardingConnector(dconn, socksTCPConn, nat)
			socksUDPConn = connect.NewLocalForwardingConnector(dconn, socksUDPConn, nat)

			stack, err := connect.NewNetworkStack(log, tunFd, tunMTU, tunNetworkAddr,
				socksTCPConn, socksUDPConn, connect.NewTransporter(log))
			if err != nil {
				return err
			}
			defer stack.Close()

			if err = parentConn.SendACK(); err != nil {
				return err
			}

			return proc.Wait()
		},
	}

	c.opts.initCliFlags(cmd)

	c.cmd = cmd
	return c
}

type runCmd struct {
	cmd  *cobra.Command
	opts runCmdOpts
}

type runCmdOpts struct {
	ForwardProxies       []string
	LocalAddressMappings []string
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
	localFlag.Value = &renamedTypeFlagValue{Value: localFlag.Value, name: "[target_host:]port:host:hostport", hideDefault: true}

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
