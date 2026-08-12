package network

import (
	"net"
	"slices"
	"strings"
)

func collectInterfaces() []Interface {
	netIfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	result := make([]Interface, 0, len(netIfaces))
	for _, ni := range netIfaces {
		iface := Interface{
			Name:      ni.Name,
			Index:     ni.Index,
			MTU:       ni.MTU,
			Flags:     parseFlags(ni.Flags.String()),
			Addresses: []string{},
		}

		addrs, err := ni.Addrs()
		if err == nil {
			for _, addr := range addrs {
				iface.Addresses = append(iface.Addresses, addr.String())
			}
		}

		result = append(result, iface)
	}

	slices.SortFunc(result, func(a, b Interface) int {
		return strings.Compare(a.Name, b.Name)
	})

	return result
}

func parseFlags(flags string) []string {
	if flags == "" {
		return nil
	}
	parts := strings.Split(flags, "|")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
