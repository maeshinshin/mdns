package main

import (
	"flag"
	"fmt"
	"net/netip"
	"os"
	"os/signal"
	"syscall"

	"github.com/maeshinshin/mdns"
)

var debug = flag.Bool("debug", false, "Enable debug mode")

func main() {
	flag.Parse()

	if *debug {
		mdns.SetDebug()
	}

	m, err := mdns.NewServer()
	if err != nil {
		panic(err)
	}
	m.Start()
	defer m.Shutdown()
	m.Register(&mdns.Service{
		Hostname: "example.local.",
		IP:       ptr(netip.MustParseAddr("192.168.1.1")),
	})

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("mDNS server running. Press Ctrl+C to exit.")
	<-sig
	m.Shutdown()
}

func ptr[T any](v T) *T {
	return &v
}
