package main

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
)

const mdnsPort = 5353

// MDNSQuerier sends unicast mDNS queries to a specific host.
type MDNSQuerier struct {
	timeout time.Duration
}

func NewMDNSQuerier(timeout time.Duration) *MDNSQuerier {
	return &MDNSQuerier{timeout: timeout}
}

// QueryHost performs full DNS-SD discovery against a single IP.
// Returns nil, nil when the host has no mDNS services.
func (q *MDNSQuerier) QueryHost(ip string) (*HostResult, error) {
	// Stage 1: enumerate service types via _services._dns-sd._udp.local.
	resp, err := q.send(ip, "_services._dns-sd._udp.local.", dns.TypePTR)
	if err != nil {
		return nil, err
	}

	addrCache := make(map[string]addrPair) // fqdn(lower) → A/AAAA
	collectAddresses(resp.Extra, addrCache)

	serviceTypes := make(map[string]struct{})
	for _, rr := range append(resp.Answer, resp.Extra...) {
		if ptr, ok := rr.(*dns.PTR); ok {
			name := ptr.Ptr
			if strings.HasSuffix(name, "._tcp.local.") || strings.HasSuffix(name, "._udp.local.") {
				serviceTypes[name] = struct{}{}
			}
		}
	}
	if len(serviceTypes) == 0 {
		return nil, nil
	}

	result := &HostResult{IP: ip}

	// Stage 2: for each service type, enumerate instances.
	instanceToType := make(map[string]string)
	for svcType := range serviceTypes {
		result.PTRAnswers = append(result.PTRAnswers, svcType)

		pResp, err := q.send(ip, svcType, dns.TypePTR)
		if err != nil {
			continue
		}
		collectAddresses(pResp.Extra, addrCache)

		for _, rr := range append(pResp.Answer, pResp.Extra...) {
			if ptr, ok := rr.(*dns.PTR); ok && ptr.Ptr != svcType {
				instanceToType[ptr.Ptr] = svcType
			}
		}
	}

	// Stage 3: resolve SRV + TXT for each instance.
	for instance, svcType := range instanceToType {
		rec := q.resolveInstance(ip, instance, svcType, addrCache)
		if rec == nil {
			continue
		}
		// Fill addresses from cache when the SRV response didn't carry them.
		hostname := strings.ToLower(rec.Hostname)
		if hostname != "" && !strings.HasSuffix(hostname, ".") {
			hostname += "."
		}
		if rec.IPv4 == "" {
			if info, ok := addrCache[hostname]; ok {
				rec.IPv4 = info.v4
			}
		}
		if rec.IPv6 == "" {
			if info, ok := addrCache[hostname]; ok {
				rec.IPv6 = info.v6
			}
		}
		if rec.IPv4 == "" {
			rec.IPv4 = ip
		}
		result.Services = append(result.Services, *rec)
	}

	return result, nil
}

// resolveInstance queries SRV and TXT records for a single service instance.
func (q *MDNSQuerier) resolveInstance(ip, instance, svcType string, addrCache map[string]addrPair) *ServiceRecord {
	rec := &ServiceRecord{}

	// Parse type/proto from the service type label, e.g. "_http._tcp.local." → http / tcp
	bare := strings.TrimSuffix(svcType, "local.")
	bare = strings.TrimSuffix(bare, ".")
	parts := strings.Split(bare, ".")
	if len(parts) >= 2 {
		rec.Type = strings.TrimPrefix(parts[len(parts)-2], "_")
		rec.Proto = strings.TrimPrefix(parts[len(parts)-1], "_")
	}

	// Instance name = everything before the service type suffix.
	instLabel := strings.TrimSuffix(instance, "."+strings.TrimPrefix(svcType, "."))
	rec.Name = unescapeDNS(instLabel)

	// Query SRV.
	srvResp, err := q.send(ip, instance, dns.TypeSRV)
	if err != nil {
		return rec
	}
	collectAddresses(srvResp.Extra, addrCache)

	var targetFQDN string
	for _, rr := range append(srvResp.Answer, srvResp.Extra...) {
		switch v := rr.(type) {
		case *dns.SRV:
			rec.Port = int(v.Port)
			rec.TTL = v.Hdr.Ttl
			rec.Hostname = strings.TrimSuffix(v.Target, ".")
			targetFQDN = strings.ToLower(v.Target)
		case *dns.A:
			rec.IPv4 = v.A.String()
		case *dns.AAAA:
			rec.IPv6 = v.AAAA.String()
		case *dns.TXT:
			rec.TXTRaw = append(rec.TXTRaw, v.Txt...)
		}
	}

	// Fill addresses from cache if the SRV additional section was empty.
	if rec.IPv4 == "" && targetFQDN != "" {
		if info, ok := addrCache[targetFQDN]; ok {
			rec.IPv4 = info.v4
			rec.IPv6 = info.v6
		}
	}

	// Query TXT if not yet obtained.
	if len(rec.TXTRaw) == 0 {
		if txtResp, err := q.send(ip, instance, dns.TypeTXT); err == nil {
			collectAddresses(txtResp.Extra, addrCache)
			for _, rr := range append(txtResp.Answer, txtResp.Extra...) {
				if txt, ok := rr.(*dns.TXT); ok {
					rec.TXTRaw = append(rec.TXTRaw, txt.Txt...)
				}
			}
		}
	}

	return rec
}

// send sends a single unicast DNS query over UDP to ip:5353.
func (q *MDNSQuerier) send(ip, name string, qtype uint16) (*dns.Msg, error) {
	m := new(dns.Msg)
	m.Id = 0 // mDNS always uses ID 0
	m.RecursionDesired = false
	m.Question = []dns.Question{
		{Name: name, Qtype: qtype, Qclass: dns.ClassINET},
	}

	buf, err := m.Pack()
	if err != nil {
		return nil, err
	}

	addr := net.JoinHostPort(ip, strconv.Itoa(mdnsPort))
	conn, err := net.DialTimeout("udp", addr, q.timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(q.timeout)) //nolint:errcheck
	if _, err := conn.Write(buf); err != nil {
		return nil, err
	}

	rbuf := make([]byte, 65535)
	n, err := conn.Read(rbuf)
	if err != nil {
		return nil, err
	}

	resp := new(dns.Msg)
	if err := resp.Unpack(rbuf[:n]); err != nil {
		return nil, err
	}
	return resp, nil
}

// addrPair holds the IPv4 and IPv6 addresses for a hostname.
type addrPair struct{ v4, v6 string }

func collectAddresses(rrs []dns.RR, cache map[string]addrPair) {
	for _, rr := range rrs {
		switch v := rr.(type) {
		case *dns.A:
			k := strings.ToLower(v.Hdr.Name)
			p := cache[k]
			p.v4 = v.A.String()
			cache[k] = p
		case *dns.AAAA:
			k := strings.ToLower(v.Hdr.Name)
			p := cache[k]
			p.v6 = v.AAAA.String()
			cache[k] = p
		}
	}
}

// unescapeDNS converts DNS label escaping (e.g. \032) back to printable chars.
func unescapeDNS(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\\' && i+3 < len(s) {
			// Numeric escape \DDD
			d0, d1, d2 := s[i+1]-'0', s[i+2]-'0', s[i+3]-'0'
			if d0 <= 9 && d1 <= 9 && d2 <= 9 {
				b.WriteByte(d0*100 + d1*10 + d2)
				i += 4
				continue
			}
		}
		if s[i] == '\\' && i+1 < len(s) {
			// Character escape \X
			b.WriteByte(s[i+1])
			i += 2
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
