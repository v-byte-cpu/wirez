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

func main() {
	var laddr, proxyFile string
	flag.StringVar(&laddr, "l", ":1080", "SOCKS5 server address")
	flag.StringVar(&proxyFile, "f", "proxies.txt", "SOCKS5 proxies file")
	flag.Parse()

	f, err := os.Open(proxyFile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	socksAddrs, err := parseProxyFile(f)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("starting listening on %s...\n", laddr)
	ln, err := net.Listen("tcp", laddr)
	if err != nil {
		log.Fatal(err)
	}
	srv := &server.Server{
		Listener: ln,
	}

	dconn := &directConnector{}
	var proxies []Connector
	for _, socksAddr := range socksAddrs {
		socksConn := newSOCKS5Connector(dconn, socksAddr)
		proxies = append(proxies, socksConn)
	}
	rotationConn := newRotationConnector(proxies)
	log.Fatal(srv.Serve(newSOCKS5ServerHandler(rotationConn)))
}
