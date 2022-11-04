# wirez

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/v-byte-cpu/wirez/blob/main/LICENSE)
[![Build Status](https://cloud.drone.io/api/badges/v-byte-cpu/wirez/status.svg)](https://cloud.drone.io/v-byte-cpu/wirez)
[![GoReportCard Status](https://goreportcard.com/badge/github.com/v-byte-cpu/wirez)](https://goreportcard.com/report/github.com/v-byte-cpu/wirez)

**wirez** can redirect all TCP/UDP traffic made by **any** given program (application, script, shell, etc.) to SOCKS5 proxy
and block other IP traffic (ICMP, SCTP etc).

Compared with [tsocks](https://linux.die.net/man/8/tsocks), [proxychains](http://proxychains.sourceforge.net/) or 
[proxychains-ng](https://github.com/rofl0r/proxychains-ng), `wirez` is not using the [LD_PRELOAD hack](https://stackoverflow.com/questions/426230/what-is-the-ld-preload-trick) 
that only works for dynamically linked programs, e.g., [applications built by Go can not be hooked by proxychains-ng](https://github.com/rofl0r/proxychains-ng/issues/199).

Instead, `wirez` is based on the rootless container technology and the userspace network stack, that is much more robust and secure.
See [how does it work](#how-does-it-work) for more details.

Also, wirez can act as a simple SOCKS5 load balancer server.

https://user-images.githubusercontent.com/65545655/200089415-fc04e91e-e933-43b6-a3b1-7243c5171f9d.mp4


---

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Load Balancing](#load-balancing)
- [How does it work?](#how-does-it-work)
- [License](#license)

---

## Installation

`wirez` is a cross-platform application. However, `run` subcommand is available only on Linux.

### From source

From the root of the source tree, run:

```
go build
```

## Quick Start

start bash shell and redirect all TCP and UDP traffic through socks5 proxy server that is listening on `127.0.0.1:1234` address:

```
wirez run -F 127.0.0.1:1234 bash
```

proxy a curl request to `example.com`:

```
wirez run -F 127.0.0.1:1234 -- curl example.com
```

By default, all UDP traffic is forwarded to SOCKS5 proxy using UDP ASSOCIATE request. 
If SOCKS5 proxy doesn't support this method (like ssh and Tor) you can use local port forwarding option `-L`.
It specifies that connections to the target host and TCP/UDP port are to be directly forwarded to the given host and port.

For instance, forward all TCP traffic through proxy, but all UDP traffic directly to 1.1.1.1 DNS server: 

```
wirez run -F 127.0.0.1:1234 -L 53:1.1.1.1:53/udp -- curl example.com
```

forward all TCP and UDP traffic through the proxy, but redirect TCP traffic targeted to `10.10.10.10:2345` directly to `127.0.0.1:4567`:

```
wirez run -F 127.0.0.1:1234 -L 10.10.10.10:2345:127.0.0.1:4567/tcp bash
```

## Load Balancing

Create a plain text file with one socks5 proxy per line. For demonstration purposes, here is an example file `proxies.txt`:

```
10.1.1.1:1035
10.2.2.2:1037
```

Start **wirez** on the localhost on port 1080:

```
wirez server -f proxies.txt -l 127.0.0.1:1080
```

Now every socks5 request on 1080 port will be load balanced between socks5 proxies in the `proxies.txt` file. Enjoy!

## Usage

```
wirez help
```

## How does it work?

First of all, `run` command creates a new unix socket pair for parent/child process communication.

```
fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
```

After that we start a new child process in a new Linux user namespace, see `user_namespaces(7)`:

```
proc.SysProcAttr = &syscall.SysProcAttr{
	Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWNET | syscall.CLONE_NEWUSER,
	...
}
```

Inside the new namespace, the child process has root capabilities, so we open a new tun network device:

```
tunFd, err := tun.Open(tunDevice)
```

and send this tun file descriptor to the parent process using the unix socket pair:

```
rights := unix.UnixRights(fd)
return unix.Sendmsg(c.socketFd, nil, rights, nil, 0)
```

in the parent process we receive this tun file descriptor:

```
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
```

and initialize a gVisor userspace network stack on top of it. Then in the child process we set up the tun device as default IP gateway 
and, finally, start a target process specified in cli args. That's it!

## License

This project is licensed under the MIT License. See the [LICENSE](https://github.com/v-byte-cpu/wirez/blob/main/LICENSE) file for the full license text.
