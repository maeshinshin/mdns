package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/maeshinshin/mdns"
	"github.com/maeshinshin/mdns/example/util"
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

	ip, err := util.GetOutboundIP()
	if err != nil {
		fmt.Println("Error getting outbound IP:", err)
		return
	}

	m.Register(&mdns.Service{
		Hostname: "example.local.",
		IP:       ip,
	})

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("mDNS server running. Press Ctrl+C to exit.")
	<-sig
	m.Shutdown()
}
