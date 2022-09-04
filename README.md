# wirez

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/v-byte-cpu/wirez/blob/main/LICENSE)
[![Build Status](https://cloud.drone.io/api/badges/v-byte-cpu/wirez/status.svg)](https://cloud.drone.io/v-byte-cpu/wirez)
[![GoReportCard Status](https://goreportcard.com/badge/github.com/v-byte-cpu/wirez)](https://goreportcard.com/report/github.com/v-byte-cpu/wirez)

**wirez** is a simple socks5 round-robin load balancer.

## Quick Start

Create a plain text file with one socks5 proxy per line. For demonstration purposes, here is an example file `proxies.txt`:

```
10.1.1.1:1035
10.2.2.2:1037
```

Start **wirez** on the localhost on port 1080:

```
wirez server -f proxies.txt -l 127.0.0.1:1080
```

Now every socks5 request on 1080 port will be load banacled between socks5 proxies in the `proxies.txt` file. Enjoy!

## Usage

```
wirez help
```