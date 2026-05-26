package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	var (
		cidr      string
		portRange string
		timeout   int
		workers   int
	)

	flag.StringVar(&cidr, "cidr", "", "IP range in CIDR notation, e.g. 192.168.1.0/24")
	flag.StringVar(&portRange, "ports", "1-65535", "Port range to include, e.g. 1-1000 or 80,443,5000")
	flag.IntVar(&timeout, "timeout", 3, "Per-host query timeout in seconds")
	flag.IntVar(&workers, "workers", 50, "Concurrent scan workers")
	flag.Parse()

	if cidr == "" {
		fmt.Fprintln(os.Stderr, "Usage: huashunxinan -cidr <CIDR> [-ports <range>] [-timeout <sec>] [-workers <n>]")
		fmt.Fprintln(os.Stderr, "  -cidr     required  192.168.1.0/24")
		fmt.Fprintln(os.Stderr, "  -ports    optional  1-65535 (default), 80,443,5000, or 1-1000")
		fmt.Fprintln(os.Stderr, "  -timeout  optional  per-host timeout seconds (default 3)")
		fmt.Fprintln(os.Stderr, "  -workers  optional  concurrent goroutines   (default 50)")
		os.Exit(1)
	}

	ips, err := expandCIDR(cidr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	portSet, err := parsePortRange(portRange)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Scanning %d hosts on port range %q ...\n", len(ips), portRange)

	scanner := NewScanner(time.Duration(timeout)*time.Second, workers)
	results := scanner.Scan(ips, portSet)

	if len(results) == 0 {
		fmt.Fprintln(os.Stderr, "No mDNS services found.")
		return
	}

	PrintResults(results)
}
