package main

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

type dnsRoundRobin struct {
	mu          sync.RWMutex
	host        string
	ips         []string
	index       atomic.Uint64
	lastResolve time.Time
	cacheTTL    time.Duration
}

func newDNSRoundRobin(host string) *dnsRoundRobin {
	return &dnsRoundRobin{
		host:     host,
		cacheTTL: 30 * time.Second,
	}
}

func (d *dnsRoundRobin) resolve() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	addrs, err := net.DefaultResolver.LookupIP(ctx, "ip", d.host)
	if err != nil {
		return err
	}

	ips := make([]string, len(addrs))
	for i, addr := range addrs {
		ips[i] = addr.String()
	}

	d.mu.Lock()
	d.ips = ips
	d.lastResolve = time.Now()
	d.mu.Unlock()

	return nil
}

func (d *dnsRoundRobin) getIP() (string, error) {
	d.mu.RLock()
	ips := d.ips
	d.mu.RUnlock()

	if len(ips) == 0 {
		if err := d.resolve(); err != nil {
			d.mu.RLock()
			ips = d.ips
			d.mu.RUnlock()
			if len(ips) == 0 {
				return "", err
			}
		} else {
			d.mu.RLock()
			ips = d.ips
			d.mu.RUnlock()
		}
	}

	if time.Since(d.lastResolve) > d.cacheTTL {
		go d.resolve()
	}

	if len(ips) == 0 {
		return "", &net.DNSError{Err: "no addresses found"}
	}

	idx := d.index.Add(1) - 1
	return ips[idx%uint64(len(ips))], nil
}

type dnsRoundTripper struct {
	dns     *dnsRoundRobin
	wrapped http.RoundTripper
	host    string
	scheme  string
	port    string
	headers map[string]string
}

func newDNSRoundTripper(rawURL string, headers ...map[string]string) (*dnsRoundTripper, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	host, port, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		host = parsed.Host
		port = ""
	}

	d := newDNSRoundRobin(host)
	if err := d.resolve(); err != nil {
		return nil, err
	}

	var hdrs map[string]string
	if len(headers) > 0 {
		hdrs = headers[0]
	}

	return &dnsRoundTripper{
		dns:     d,
		wrapped: http.DefaultTransport,
		host:    parsed.Host,
		scheme:  parsed.Scheme,
		port:    port,
		headers: hdrs,
	}, nil
}

func (d *dnsRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ip, err := d.dns.getIP()
	if err != nil {
		return nil, err
	}

	host := ip
	if d.port != "" {
		host = ip + ":" + d.port
	}

	req.URL.Scheme = d.scheme
	req.URL.Host = host
	req.Host = d.host

	for k, v := range d.headers {
		req.Header.Set(k, v)
	}

	return d.wrapped.RoundTrip(req)
}
