package mdns

import (
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"

	"golang.org/x/net/dns/dnsmessage"
)

const (
	mdnsMulticastAddressStr = "224.0.0.251"
	mdnsPort                = 5353
	defaultTTL              = 120
)

var (
	mdnsMulticastAddress = netip.MustParseAddr(mdnsMulticastAddressStr)
	logger               = slog.New(slog.NewTextHandler(os.Stdout, nil))
)

type op int

func NewServer() (*Server, error) {
	var conn *net.UDPConn
	var destAddr *net.UDPAddr
	var err error

	if destAddr, err = net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", mdnsMulticastAddressStr, mdnsPort)); err != nil {
		return nil, fmt.Errorf("failed to resolve mDNS address: %w", err)
	}

	if conn, err = net.ListenMulticastUDP("udp4", nil, destAddr); err != nil {
		return nil, fmt.Errorf("failed to create mDNS server: %w", err)
	}

	return &Server{
		services:  make(map[string]*Service),
		records:   make(map[*Service]*dnsmessage.Resource),
		conn:      conn,
		destAddr:  destAddr,
		shutdown:  make(chan struct{}),
		opChannel: make(chan operation),
	}, nil
}

func (s *Server) Register(srvice *Service) error {
	s.opChannel <- operation{
		op:      registerOp,
		service: srvice,
	}
	return nil
}

func (s *Server) Unregister(hostname string, IP *netip.Addr) error {
	s.opChannel <- operation{
		op: unregisterOp,
		service: &Service{
			Hostname: hostname,
			IP:       IP,
		},
	}
	return nil
}

func (s *Server) Start() {
	go s.run()
	go s.listen()
}

func (s *Server) listen() {
	logger.Info("Starting mDNS listener", "address", s.conn.LocalAddr().String())
	buf := make([]byte, 1500)
	for {
		select {
		case <-s.shutdown:
			return
		default:
			n, addr, err := s.conn.ReadFromUDP(buf)
			if err != nil {
				select {
				case <-s.shutdown:
					return
				default:
					logger.Error("Failed to read from UDP", "error", err)
				}
				continue
			}

			s.handleQuery(buf[:n], addr)
		}
	}
}

func (s *Server) handleQuery(packet []byte, from *net.UDPAddr) {
	var p dnsmessage.Parser
	header, err := p.Start(packet)
	if err != nil {
		logger.Error("Failed to parse DNS message header", "error", err)
		return
	}

	if header.Response {
		return
	}

	questions, err := p.AllQuestions()
	if err != nil {
		logger.Error("Failed to parse DNS questions", "error", err)
		return
	}

	if len(questions) == 0 {
		logger.Warn("Received DNS query with no questions", "from", from.String())
		return
	}

	logger.Debug(fmt.Sprintf("Received query from %s for: %v", from.String(), questions))
	var answers []dnsmessage.Resource
	s.mu.Lock()
	for _, q := range questions {
		if q.Type != dnsmessage.TypeA && q.Type != dnsmessage.TypeAAAA {
			continue
		}
		if service, ok := s.services[q.Name.String()]; ok {
			if record, ok := s.records[service]; ok {
				if record.Header.Type == q.Type {
					answers = append(answers, *record)
				}
			}
		}
	}
	s.mu.Unlock()

	if len(answers) > 0 {
		s.sendResponse(answers, from)
	}
}

func (s *Server) sendResponse(answers []dnsmessage.Resource, to *net.UDPAddr) {
	msg := dnsmessage.Message{
		Header: dnsmessage.Header{
			Response:      true,
			Authoritative: true,
		},
		Answers: answers,
	}

	packed, err := msg.Pack()
	if err != nil {
		logger.Error("Failed to pack DNS response", "error", err)
		return
	}

	logger.Info("Sending DNS response", "answers", len(answers), "to", to.String())
	_, err = s.conn.WriteToUDP(packed, to)
	if err != nil {
		logger.Error("Failed to send DNS response", "error", err, "to", to.String())
	}
}

func (s *Server) run() {
	logger.Info("Starting mDNS operational loop", "address", mdnsMulticastAddressStr, "port", mdnsPort)
	for {
		select {
		case op := <-s.opChannel:
			s.mu.Lock()
			switch op.op {
			case registerOp:
				logger.Info("Registering service", "hostname", op.service.Hostname, "ip", op.service.IP)
				s.services[op.service.Hostname] = op.service

				record, err := buildServiceRecord(op.service, defaultTTL)
				if err != nil {
					logger.Error("Failed to build service record", "error", err, "service", op.service.Hostname)
					continue
				}
				s.records[op.service] = record

			case unregisterOp:
				logger.Info("Unregistering service", "hostname", op.service.Hostname, "ip", op.service.IP)
				if service, exists := s.services[op.service.Hostname]; exists {
					s.sendGoodbye(service)
					delete(s.records, service)
					delete(s.services, op.service.Hostname)
				} else {
					logger.Warn("Attempted to unregister non-existent service", "hostname", op.service.Hostname, "ip", op.service.IP)
				}
			}
			s.mu.Unlock()

		case <-s.shutdown:
			logger.Info("Stopping mDNS operational loop")
			s.conn.Close()
			return
		}
	}
}

func (s *Server) sendGoodbye(service *Service) {
	record, err := buildServiceRecord(service, 0)
	if err != nil {
		logger.Error("Failed to build goodbye record", "error", err, "service", service.Hostname)
		return
	}

	msg := dnsmessage.Message{
		Header: dnsmessage.Header{
			Response:      true,
			Authoritative: true,
		},
		Answers: []dnsmessage.Resource{*record},
	}
	packed, err := msg.Pack()
	if err != nil {
		logger.Error("Failed to pack goodbye message", "error", err, "service", service.Hostname)
		return
	}
	logger.Debug("Sending goodbye message", "service", service.Hostname, "ip", service.IP)
	_, err = s.conn.WriteToUDP(packed, s.destAddr)
	if err != nil {
		logger.Error("Failed to send goodbye message", "error", err, "service", service.Hostname)
	}
}

func (s *Server) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger.Info("Shutting down mDNS server...")
	for _, service := range s.services {
		s.sendGoodbye(service)
	}

	s.shutdown <- struct{}{}
}
