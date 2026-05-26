package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Scanner drives parallel mDNS discovery over a list of IPs.
type Scanner struct {
	querier *MDNSQuerier
	workers int
}

func NewScanner(timeout time.Duration, workers int) *Scanner {
	return &Scanner{
		querier: NewMDNSQuerier(timeout),
		workers: workers,
	}
}

// Scan queries every IP concurrently and filters services by the given port set.
// A nil portSet means "accept all ports".
func (s *Scanner) Scan(ips []string, portSet map[int]struct{}) []*HostResult {
	type work struct{ ip string }
	jobs := make(chan work, len(ips))
	for _, ip := range ips {
		jobs <- work{ip}
	}
	close(jobs)

	var (
		mu      sync.Mutex
		results []*HostResult
		wg      sync.WaitGroup
	)

	workers := s.workers
	if workers > len(ips) {
		workers = len(ips)
	}
	for range make([]struct{}, workers) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				res, err := s.querier.QueryHost(j.ip)
				if err != nil || res == nil {
					continue
				}
				// Filter services by port set.
				if portSet != nil {
					var filtered []ServiceRecord
					for _, svc := range res.Services {
						// port 0 = no-port services (e.g. device-info); always include.
						if svc.Port == 0 {
							filtered = append(filtered, svc)
							continue
						}
						if _, ok := portSet[svc.Port]; ok {
							filtered = append(filtered, svc)
						}
					}
					res.Services = filtered
				}
				if len(res.Services) == 0 {
					continue
				}
				mu.Lock()
				results = append(results, res)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	// Deterministic ordering: by IP address.
	sort.Slice(results, func(i, j int) bool {
		return ipToUint32(results[i].IP) < ipToUint32(results[j].IP)
	})
	return results
}

// expandCIDR returns all usable host IPs in the given CIDR block.
func expandCIDR(cidr string) ([]string, error) {
	ip, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
	}

	// Single host notation (e.g. "192.168.1.5/32").
	ones, bits := network.Mask.Size()
	if ones == bits {
		return []string{ip.String()}, nil
	}

	var ips []string
	for cur := cloneIP(network.IP); network.Contains(cur); inc(cur) {
		// Skip network and broadcast addresses for IPv4 /prefix < 31.
		if ones < 31 {
			last := lastIP(network)
			if cur.Equal(network.IP) || cur.Equal(last) {
				continue
			}
		}
		ips = append(ips, cur.String())
	}
	return ips, nil
}

// parsePortRange parses "1-1024", "80,443,8080", or combinations thereof.
// Returns nil to mean "all ports".
func parsePortRange(s string) (map[int]struct{}, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "1-65535" {
		return nil, nil
	}

	portSet := make(map[int]struct{})
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if idx := strings.Index(part, "-"); idx != -1 {
			lo, err := strconv.Atoi(part[:idx])
			if err != nil {
				return nil, fmt.Errorf("invalid port %q", part[:idx])
			}
			hi, err := strconv.Atoi(part[idx+1:])
			if err != nil {
				return nil, fmt.Errorf("invalid port %q", part[idx+1:])
			}
			if lo > hi || lo < 1 || hi > 65535 {
				return nil, fmt.Errorf("invalid port range %q", part)
			}
			for p := lo; p <= hi; p++ {
				portSet[p] = struct{}{}
			}
		} else {
			p, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid port %q", part)
			}
			if p < 1 || p > 65535 {
				return nil, fmt.Errorf("port %d out of range", p)
			}
			portSet[p] = struct{}{}
		}
	}
	return portSet, nil
}

// --- helpers ---

func cloneIP(ip net.IP) net.IP {
	c := make(net.IP, len(ip))
	copy(c, ip)
	return c
}

func inc(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}

func lastIP(n *net.IPNet) net.IP {
	last := make(net.IP, len(n.IP))
	for i := range n.IP {
		last[i] = n.IP[i] | ^n.Mask[i]
	}
	return last
}

func ipToUint32(s string) uint32 {
	ip := net.ParseIP(s).To4()
	if ip == nil {
		return 0
	}
	return binary.BigEndian.Uint32(ip)
}
