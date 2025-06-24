package mdns

import (
	"fmt"
	"log/slog"
	"os"

	"golang.org/x/net/dns/dnsmessage"
)

func SetDebug() {
	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

func buildServiceRecord(service *Service, ttl uint32) (*dnsmessage.Resource, error) {
	if service == nil {
		return nil, fmt.Errorf("service cannot be nil")
	}

	var host dnsmessage.Name
	var err error

	if host, err = dnsmessage.NewName(service.Hostname); err != nil {
		return nil, fmt.Errorf("invalid hostname %q: %w", service.Hostname, err)
	}

	if service.IP.Is4() {
		return buildARecord(service, host, ttl), nil
	} else if service.IP.Is6() {
		return buildAAAARecord(service, host, ttl), nil
	}
	return nil, fmt.Errorf("unsupported IP version: %s", service.IP)
}

func buildARecord(service *Service, host dnsmessage.Name, ttl uint32) *dnsmessage.Resource {
	return &dnsmessage.Resource{
		Header: dnsmessage.ResourceHeader{
			Name:  host,
			Type:  dnsmessage.TypeA,
			Class: dnsmessage.ClassINET | (1 << 15),
			TTL:   ttl,
		},
		Body: &dnsmessage.AResource{
			A: service.IP.As4(),
		},
	}
}

func buildAAAARecord(service *Service, host dnsmessage.Name, ttl uint32) *dnsmessage.Resource {
	return &dnsmessage.Resource{
		Header: dnsmessage.ResourceHeader{
			Name:  host,
			Type:  dnsmessage.TypeAAAA,
			Class: dnsmessage.ClassINET | (1 << 15),
			TTL:   ttl,
		},
		Body: &dnsmessage.AAAAResource{
			AAAA: service.IP.As16(),
		},
	}
}
