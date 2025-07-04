package util

import (
	"net"
	"net/netip"
)

func GetOutboundIP() (*netip.Addr, error) {
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	ip, err := netip.ParseAddr(conn.LocalAddr().(*net.UDPAddr).IP.String())
	if err != nil {
		return nil, err
	}

	return &ip, nil
}
