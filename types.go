package main

// ServiceRecord represents a single discovered mDNS service instance.
type ServiceRecord struct {
	Port     int
	Proto    string // "tcp" or "udp"
	Type     string // e.g. "http", "smb", "workstation"
	Name     string // instance name, e.g. "slw-nas [24:5e:be:69:a3:13]"
	IPv4     string
	IPv6     string
	Hostname string // e.g. "slw-nas.local"
	TTL      uint32
	TXTRaw   []string // raw TXT strings as returned by the record
}

// HostResult holds all mDNS discovery results for a single host.
type HostResult struct {
	IP         string
	Services   []ServiceRecord
	PTRAnswers []string // service types found via DNS-SD
}
