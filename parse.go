package main

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/url"
	"strings"
)

func parseProxyFile(proxyFile io.Reader) (socksAddrs []*SocksAddr, err error) {
	bs := bufio.NewScanner(proxyFile)
	for bs.Scan() {
		rawSocksAddr := strings.Trim(bs.Text(), " ")
		if rawSocksAddr == "" || rawSocksAddr[0] == '#' {
			continue
		}
		if !strings.Contains(rawSocksAddr, "//") {
			rawSocksAddr = "socks5://" + rawSocksAddr
		}
		socksURL, err := url.Parse(rawSocksAddr)
		if err != nil {
			return nil, err
		}
		if socksURL.Scheme != "socks5" {
			return nil, errors.New("invalid socks5 scheme")
		}
		if _, _, err := net.SplitHostPort(socksURL.Host); err != nil {
			return nil, err
		}
		socksAddrs = append(socksAddrs, &SocksAddr{Address: socksURL.Host, Auth: socksURL.User})
	}
	err = bs.Err()
	return
}
