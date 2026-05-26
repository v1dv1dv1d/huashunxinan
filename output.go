package main

import (
	"fmt"
	"sort"
	"strings"
)

// PrintResults writes all discovered hosts to stdout in the canonical format.
func PrintResults(results []*HostResult) {
	for _, r := range results {
		fmt.Printf("# Host: %s\n", r.IP)
		printHostResult(r)
		fmt.Println()
	}
}

func printHostResult(r *HostResult) {
	// Sort services: by port asc, then by type name.
	svcs := make([]ServiceRecord, len(r.Services))
	copy(svcs, r.Services)
	sort.Slice(svcs, func(i, j int) bool {
		if svcs[i].Port != svcs[j].Port {
			return svcs[i].Port < svcs[j].Port
		}
		return svcs[i].Type < svcs[j].Type
	})

	fmt.Println("services:")
	for _, svc := range svcs {
		printService(svc)
	}

	// PTR answers section.
	if len(r.PTRAnswers) > 0 {
		ptrs := make([]string, len(r.PTRAnswers))
		copy(ptrs, r.PTRAnswers)
		sort.Strings(ptrs)

		fmt.Println("answers:")
		fmt.Println("PTR:")
		for _, p := range ptrs {
			// Strip trailing dot and "local." to show the bare service label.
			fmt.Printf("%s\n", strings.TrimSuffix(p, "."))
		}
	}
}

func printService(svc ServiceRecord) {
	// Header line: "9/tcp workstation:" or "device-info:" when port is 0.
	if svc.Port > 0 {
		fmt.Printf("%d/%s %s:\n", svc.Port, svc.Proto, svc.Type)
	} else {
		fmt.Printf("%s:\n", svc.Type)
	}

	fmt.Printf("Name=%s\n", svc.Name)
	if svc.IPv4 != "" {
		fmt.Printf("IPv4=%s\n", svc.IPv4)
	}
	if svc.IPv6 != "" {
		fmt.Printf("IPv6=%s\n", svc.IPv6)
	}
	if svc.Hostname != "" {
		fmt.Printf("Hostname=%s\n", svc.Hostname)
	}
	fmt.Printf("TTL=%d\n", svc.TTL)

	// TXT data — each raw string on its own line (skip empty / single-dot markers).
	for _, t := range svc.TXTRaw {
		if t != "" && t != "." {
			fmt.Println(t)
		}
	}
}
