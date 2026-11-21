package tunnel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// target is a dialable tunnel server — the unit of one tunnel connection.
type target struct {
	host string
	port int
	// servername is the TLS SNI / verification name. For SRV-discovered targets
	// this is the SRV QUERY name (what the cloud's cert covers), regardless of
	// which per-record host is dialed; for explicit addresses it is the host.
	servername string
	// plaintext dials plaintext h2 (set for an explicit http:// server).
	plaintext bool
}

// key is the stable identity of a target (host:port) — the slot key.
func (t target) key() string { return net.JoinHostPort(t.host, strconv.Itoa(t.port)) }

func (t target) String() string { return t.key() }

// parseServer parses one explicit tunnel-server address: "host:port" (TLS),
// "https://host:port" (TLS), or "http://host:port" (plaintext h2).
func parseServer(s string) (target, error) {
	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err != nil {
			return target{}, fmt.Errorf("tunnel: invalid server %q: %w", s, err)
		}
		if (u.Path != "" && u.Path != "/") || u.RawQuery != "" {
			return target{}, fmt.Errorf("tunnel: server %q must not have a path or query", s)
		}
		host := u.Hostname()
		if host == "" {
			return target{}, fmt.Errorf("tunnel: server %q is missing a host", s)
		}
		portStr := u.Port()
		switch u.Scheme {
		case "http":
			if portStr == "" {
				portStr = "80"
			}
			port, err := parsePort(portStr, s)
			if err != nil {
				return target{}, err
			}
			return target{host: host, port: port, servername: host, plaintext: true}, nil
		case "https":
			if portStr == "" {
				portStr = "443"
			}
			port, err := parsePort(portStr, s)
			if err != nil {
				return target{}, err
			}
			return target{host: host, port: port, servername: host}, nil
		default:
			return target{}, fmt.Errorf("tunnel: server %q has unsupported scheme %q", s, u.Scheme)
		}
	}

	host, portStr, err := net.SplitHostPort(s)
	if err != nil {
		return target{}, fmt.Errorf("tunnel: server %q must be host:port: %w", s, err)
	}
	port, err := parsePort(portStr, s)
	if err != nil {
		return target{}, err
	}
	return target{host: host, port: port, servername: host}, nil
}

func parsePort(s, addr string) (int, error) {
	port, err := strconv.Atoi(s)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("tunnel: invalid port in server %q", addr)
	}
	return port, nil
}

// resolveSRVTargets resolves a DNS SRV name to a per-IP target set: SRV records
// (priority asc, weight desc) each expanded to ALL of their A/AAAA addresses, one
// connection per unique address:port. TLS SNI uses the SRV query name (the
// cloud's cert covers the SRV name, not per-node hostnames).
//
// Error taxonomy mirrors the Rust/TS resolver: a NEGATIVE answer for an SRV
// record (the name genuinely has no address) drops that record, and an
// all-negative answer yields an EMPTY set (the supervisor reconciles slots away).
// A TRANSPORT error (timeout/servfail) returns an error instead, so the caller
// keeps existing connections serving and retries rather than tearing down healthy
// slots over a resolver blip.
func resolveSRVTargets(ctx context.Context, resolver *net.Resolver, srvName string) ([]target, error) {
	_, recs, err := resolver.LookupSRV(ctx, "", "", srvName)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(recs, func(i, j int) bool {
		if recs[i].Priority != recs[j].Priority {
			return recs[i].Priority < recs[j].Priority
		}
		return recs[i].Weight > recs[j].Weight
	})

	// Look up every record's addresses concurrently so one slow resolver doesn't
	// serialize the rest.
	type lookup struct {
		addrs []net.IPAddr
		err   error
	}
	results := make([]lookup, len(recs))
	var wg sync.WaitGroup
	for i, r := range recs {
		wg.Add(1)
		go func(i int, host string) {
			defer wg.Done()
			addrs, err := resolver.LookupIPAddr(ctx, host)
			results[i] = lookup{addrs: addrs, err: err}
		}(i, strings.TrimSuffix(r.Target, "."))
	}
	wg.Wait()

	var targets []target
	seen := make(map[string]bool)
	for i, r := range recs {
		res := results[i]
		if res.err != nil {
			if isNotFoundDNS(res.err) {
				continue // negative answer: this record genuinely has no address
			}
			return nil, res.err // transport error: keep existing slots, retry
		}
		for _, a := range res.addrs {
			key := net.JoinHostPort(a.IP.String(), strconv.Itoa(int(r.Port)))
			if seen[key] {
				continue
			}
			seen[key] = true
			targets = append(targets, target{host: a.IP.String(), port: int(r.Port), servername: srvName})
		}
	}
	return targets, nil
}

func isNotFoundDNS(err error) bool {
	var d *net.DNSError
	return errors.As(err, &d) && d.IsNotFound
}
