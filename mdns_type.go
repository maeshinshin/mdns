package mdns

import (
	"net"
	"net/netip"
	"sync"

	"golang.org/x/net/dns/dnsmessage"
)

type Service struct {
	Hostname string
	IP       *netip.Addr
}

type Server struct {
	services  map[string]*Service
	records   map[*Service]*dnsmessage.Resource
	mu        sync.Mutex
	conn      *net.UDPConn
	destAddr  *net.UDPAddr
	shutdown  chan struct{}
	opChannel chan operation
}

type opKind int

const (
	registerOp opKind = iota
	unregisterOp
)

type operation struct {
	op      opKind
	service *Service
}
