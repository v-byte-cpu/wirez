package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/ginuerzh/gosocks5/server"
)

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

	dconn := &directConnector{}
	proxies := make([]Connector, 0, len(socksAddrs))
	for _, socksAddr := range socksAddrs {
		socksConn := newSOCKS5Connector(dconn, socksAddr)
		proxies = append(proxies, socksConn)
	}
	rotationConn := newRotationConnector(proxies)

	go func() {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()
		<-ctx.Done()
		if err := srv.Close(); err != nil {
			log.Println(err)
		}
	}()

	err = srv.Serve(newSOCKS5ServerHandler(rotationConn))
	if err != nil && !errors.Is(err, net.ErrClosed) {
		return err
	}
	return nil
}

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}
