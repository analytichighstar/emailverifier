package validator

import (
	"context"
	"net"
	"time"
)

// DNSResolver interface for making DNS lookups configurable and mockable
type DNSResolver interface {
	LookupHost(domain string) ([]string, error)
	LookupMX(domain string) ([]*net.MX, error)
}

// DefaultResolver implements DNSResolver using net package with context timeouts
type DefaultResolver struct {
	timeout time.Duration
}

// LookupHost performs a DNS lookup for the given domain and returns a list of IP addresses.
func (r *DefaultResolver) LookupHost(domain string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	return net.DefaultResolver.LookupHost(ctx, domain)
}

// LookupMX performs a DNS lookup for MX records of the given domain.
func (r *DefaultResolver) LookupMX(domain string) ([]*net.MX, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	return net.DefaultResolver.LookupMX(ctx, domain)
}
