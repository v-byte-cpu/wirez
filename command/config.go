package command

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/pflag"
	"github.com/v-byte-cpu/wirez/pkg/connect"
)

func parseProxyFile(proxyFile io.Reader) (socksAddrs []*connect.SocksAddr, err error) {
	bs := bufio.NewScanner(proxyFile)
	for bs.Scan() {
		rawSocksAddr := strings.Trim(bs.Text(), " ")
		if rawSocksAddr == "" || rawSocksAddr[0] == '#' {
			continue
		}
		socksAddr, err := parseProxyURL(rawSocksAddr)
		if err != nil {
			return nil, err
		}
		socksAddrs = append(socksAddrs, socksAddr)
	}
	err = bs.Err()
	return
}

func parseProxyURL(proxyURL string) (*connect.SocksAddr, error) {
	proxyURL = strings.Trim(proxyURL, " ")
	if !strings.Contains(proxyURL, "//") {
		proxyURL = "socks5://" + proxyURL
	}
	socksURL, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}
	if socksURL.Scheme != "socks5" {
		return nil, errors.New("invalid socks5 scheme")
	}
	if _, _, err := net.SplitHostPort(socksURL.Host); err != nil {
		return nil, err
	}
	return &connect.SocksAddr{Address: socksURL.Host, Auth: socksURL.User}, nil
}

func parseProxyURLs(proxyURLs []string) ([]*connect.SocksAddr, error) {
	result := make([]*connect.SocksAddr, 0, len(proxyURLs))
	for _, proxyURL := range proxyURLs {
		socksAddr, err := parseProxyURL(proxyURL)
		if err != nil {
			return nil, err
		}
		result = append(result, socksAddr)
	}
	return result, nil
}

func parseAddressMapper(addressMappings []string) (connect.AddressMapper, error) {
	m := connect.NewAddressMapper()
	for _, mapping := range addressMappings {
		network, fromAddress, targetAddress, err := parseMapping(mapping)
		if err != nil {
			return nil, err
		}
		if err = m.AddAddressMapping(network, fromAddress, targetAddress); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func parseMapping(mapping string) (network, fromAddress, targetAddress string, err error) {
	parts := strings.Split(mapping, "/")
	if len(parts) < 2 {
		network = "tcp"
	} else {
		network = parts[1]
	}
	targetPort, rest, err := takeLastPort(parts[0])
	if err != nil {
		err = fmt.Errorf("invalid target port in mapping %s: %w", mapping, err)
		return
	}
	targetHost, rest, err := takeLastHost(rest)
	if err != nil {
		err = fmt.Errorf("invalid target host in mapping %s: %w", mapping, err)
		return
	}
	if len(targetHost) == 0 {
		err = fmt.Errorf("empty target host in mapping %s", mapping)
		return
	}
	fromPort, rest, err := takeLastPort(rest)
	if err != nil {
		err = fmt.Errorf("invalid source port in mapping %s: %w", mapping, err)
		return
	}
	fromHost, rest, err := takeLastHost(rest)
	if err != nil {
		err = fmt.Errorf("invalid source host in mapping %s: %w", mapping, err)
		return
	}
	if len(rest) > 0 {
		err = fmt.Errorf("invalid source address in mapping %s", mapping)
		return
	}
	fromAddress = net.JoinHostPort(fromHost, fromPort)
	targetAddress = net.JoinHostPort(targetHost, targetPort)
	return
}

func takeLastHost(input string) (host, rest string, err error) {
	if len(input) == 0 {
		return
	}
	if input[len(input)-1] == ']' {
		return takeLastIPv6Host(input)
	}
	idx := strings.LastIndex(input, ":")
	host = input[idx+1:]
	if idx > 0 {
		rest = input[:idx]
	}
	return host, rest, err
}

func takeLastIPv6Host(input string) (host, rest string, err error) {
	idx := strings.LastIndex(input, "[")
	if idx == -1 {
		return "", "", errors.New("invalid IPv6 address")
	}
	host = input[idx+1 : len(input)-1]
	if idx > 0 {
		if input[idx-1] != ':' {
			return "", "", errors.New("missing colon before host")
		}
		rest = input[:idx-1]
	}
	if ip := net.ParseIP(host); ip == nil {
		err = errors.New("invalid IPv6 address")
	}
	return host, rest, err
}

func takeLastPort(input string) (port, rest string, err error) {
	idx := strings.LastIndex(input, ":")
	port = input[idx+1:]
	if idx > 0 {
		rest = input[:idx]
	}
	_, err = strconv.ParseUint(port, 10, 16)
	return
}

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
	case verboseLevel == 1:
		level = zerolog.DebugLevel
	case verboseLevel >= 2:
		level = zerolog.TraceLevel
	}
	result := log.Level(level)
	return &result
}
