//go:build linux

package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"gvisor.dev/gvisor/pkg/tcpip/link/rawfile"
	"gvisor.dev/gvisor/pkg/tcpip/link/tun"
)

const (
	loDevice       = "lo"
	tunDevice      = "tun0"
	tunNetworkAddr = "10.1.1.1/24"
)

func newRunContainerCmd() *runContainerCmd {
	c := &runContainerCmd{}

	cmd := &cobra.Command{
		Use:     "runc [flags] command",
		Example: "runc --hostname=wirez --unix-fd=10 bash",
		Short:   "Internal command to run a new process inside an isolated container",
		Hidden:  true,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if err = syscall.Sethostname([]byte(c.opts.Hostname)); err != nil {
				return
			}

			childConn := newChildUnixSocketConn(c.opts.PipeFd)
			defer childConn.Close()

			tunFd, err := tun.Open(tunDevice)
			if err != nil {
				return err
			}
			defer unix.Close(tunFd)

			if err = childConn.SendFd(tunFd); err != nil {
				return
			}

			mtu, err := rawfile.GetMTU(tunDevice)
			if err != nil {
				return fmt.Errorf("get mtu: %w", err)
			}

			if err = childConn.SendMTU(mtu); err != nil {
				return
			}

			// wait for starting network stack
			if err = childConn.ReceiveACK(); err != nil {
				return
			}

			if err = setupIPNetwork(); err != nil {
				return err
			}

			proc := exec.Command(args[0], args[1:]...)
			proc.Stdin = os.Stdin
			proc.Stdout = os.Stdout
			proc.Stderr = os.Stderr

			if c.opts.Privileged {
				proc.SysProcAttr = &syscall.SysProcAttr{
					Credential: &syscall.Credential{Uid: uint32(c.opts.ContainerUID), Gid: uint32(c.opts.ContainerGID)},
				}
			} else if c.opts.ContainerUID != 0 {
				proc.SysProcAttr = &syscall.SysProcAttr{
					Cloneflags: syscall.CLONE_NEWUSER,
					Credential: &syscall.Credential{Uid: uint32(c.opts.ContainerUID), Gid: uint32(c.opts.ContainerGID)},
					UidMappings: []syscall.SysProcIDMap{
						{ContainerID: c.opts.ContainerUID, HostID: os.Geteuid(), Size: 1},
					},
					GidMappings: []syscall.SysProcIDMap{
						{ContainerID: c.opts.ContainerGID, HostID: os.Getegid(), Size: 1},
					},
				}
			}

			return proc.Run()
		},
	}

	c.opts.initCliFlags(cmd)

	c.cmd = cmd
	return c
}

type runContainerCmd struct {
	cmd  *cobra.Command
	opts runContainerCmdOpts
}

type runContainerCmdOpts struct {
	Hostname     string
	PipeFd       int
	ContainerUID int
	ContainerGID int
	Privileged   bool
}

func (o *runContainerCmdOpts) initCliFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.Hostname, "hostname", "wirez", "set container hostname")
	cmd.Flags().IntVar(&o.PipeFd, "unix-fd", 0, "set unix pipe fd")
	cmd.Flags().IntVar(&o.ContainerUID, "uid", os.Geteuid(), "set uid of container process")
	cmd.Flags().IntVar(&o.ContainerGID, "gid", os.Getegid(), "set gid of container process")
	cmd.Flags().BoolVar(&o.Privileged, "privileged", false, "indicates if started with root privileges")
}

type childUnixSocketConn struct {
	socketFd   int
	socketFile *os.File
}

func newChildUnixSocketConn(socketFd int) *childUnixSocketConn {
	return &childUnixSocketConn{
		socketFd:   socketFd,
		socketFile: os.NewFile(uintptr(socketFd), "childPipe"),
	}
}

func (c *childUnixSocketConn) Close() error {
	return unix.Close(c.socketFd)
}

func (c *childUnixSocketConn) SendFd(fd int) error {
	rights := unix.UnixRights(fd)
	return unix.Sendmsg(c.socketFd, nil, rights, nil, 0)
}

func (c *childUnixSocketConn) SendMTU(mtu uint32) error {
	data, err := json.Marshal(&MTUMessage{MTU: mtu})
	if err != nil {
		return err
	}
	_, err = c.socketFile.Write(data)
	return err
}

func (c *childUnixSocketConn) ReceiveACK() (err error) {
	var msg ACKMessage
	if err = json.NewDecoder(c.socketFile).Decode(&msg); err != nil {
		return
	}
	if !msg.ACK {
		return errors.New("network stack initialization is not acknowledged")
	}
	return
}

type MTUMessage struct {
	MTU uint32 `json:"mtu"`
}

func setupIPNetwork() error {
	lo, err := netlink.LinkByName(loDevice)
	if err != nil {
		return err
	}
	if err = netlink.LinkSetUp(lo); err != nil {
		return err
	}

	tun0, tunAddr, err := setupIPAddress(tunDevice, tunNetworkAddr)
	if err != nil {
		return err
	}

	return netlink.RouteAdd(&netlink.Route{
		Gw:        tunAddr.IP,
		LinkIndex: tun0.Attrs().Index,
	})
}

func setupIPAddress(device, networkAddr string) (dev netlink.Link, addr *netlink.Addr, err error) {
	dev, err = netlink.LinkByName(device)
	if err != nil {
		return
	}
	if err = netlink.LinkSetUp(dev); err != nil {
		return
	}
	addr, err = netlink.ParseAddr(networkAddr)
	if err != nil {
		return
	}
	err = netlink.AddrAdd(dev, addr)
	return
}
