# wirez

**wirez** is a simple socks5 round-robin load balancer.

## Quick Start

Create a plain text file with one socks5 proxy per line. For demonstration purposes, here is an example file `proxies.txt`:

```
10.1.1.1:1035
10.2.2.2:1037
```

Start **wirez** on the localhost on port 1080:

```
wirez -f proxies.txt -l 127.0.0.1:1080
```

Now every socks5 request on 1080 port will be load banacled between socks5 proxies in the `proxies.txt` file. Enjoy!

## Usage

```
$ ./wirez -h
Usage of ./wirez:
  -f string
        SOCKS5 proxies file (default "proxies.txt")
  -l string
        SOCKS5 server address (default ":1080")
```