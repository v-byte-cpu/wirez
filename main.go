package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"net"
	"os"

	"github.com/ginuerzh/gosocks5/server"
)

func parseProxyFile(proxyFile io.Reader) (socksAddrs []string, err error) {
	bs := bufio.NewScanner(proxyFile)
	for bs.Scan() {
		socksAddrs = append(socksAddrs, bs.Text())
	}
	err = bs.Err()
	return
}

func Main() error {
	var laddr, proxyFile string
	flag.StringVar(&laddr, "l", ":1080", "SOCKS5 server address")
	flag.StringVar(&proxyFile, "f", "proxies.txt", "SOCKS5 proxies file")
	flag.Parse()

	f, err := os.Open(proxyFile)
	if err != nil {
		return err
	}
	defer f.Close()
	socksAddrs, err := parseProxyFile(f)
	if err != nil {
		return err
	}

	log.Printf("starting listening on %s...\n", laddr)
	ln, err := net.Listen("tcp", laddr)
	if err != nil {
		return err
	}
	srv := &server.Server{
		Listener: ln,
	}
	// TODO srv.Close()

	dconn := &directConnector{}
	proxies := make([]Connector, 0, len(socksAddrs))
	for _, socksAddr := range socksAddrs {
		socksConn := newSOCKS5Connector(dconn, socksAddr)
		proxies = append(proxies, socksConn)
	}
	rotationConn := newRotationConnector(proxies)
	return srv.Serve(newSOCKS5ServerHandler(rotationConn))
}

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}
