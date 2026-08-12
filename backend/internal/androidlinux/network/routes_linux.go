//go:build linux && !android

package network

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
)

func collectRoutes() []Route {
	var routes []Route
	routes = append(routes, parseProcNetRoute("/proc/net/route", "ipv4")...)
	routes = append(routes, parseProcNetIPv6Route("/proc/net/ipv6_route", "ipv6")...)

	slices.SortFunc(routes, func(a, b Route) int {
		return strings.Compare(a.Interface, b.Interface)
	})

	return routes
}

func parseProcNetRoute(path, family string) []Route {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var routes []Route
	scanner := bufio.NewScanner(file)
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			first = false
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		dest, err := parseHexIPv4(fields[1])
		if err != nil {
			continue
		}
		gw, _ := parseHexIPv4(fields[7])
		mask, err := parseHexIPv4(fields[3])
		if err != nil {
			continue
		}
		prefix := maskToPrefix(mask)
		metric, _ := strconv.Atoi(fields[6])

		routes = append(routes, Route{
			Interface:   strings.TrimSpace(fields[0]),
			Destination: dest,
			Gateway:     gw,
			Prefix:      prefix,
			Metric:      metric,
			Family:      family,
		})
	}
	return routes
}

func parseProcNetIPv6Route(path, family string) []Route {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var routes []Route
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			continue
		}
		dest := parseHexIPv6(fields[0])
		prefix, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		gw := parseHexIPv6(fields[4])
		metric, err := strconv.ParseInt(fields[5], 16, 32)
		if err != nil {
			continue
		}
		iface := fields[9]

		routes = append(routes, Route{
			Interface:   strings.TrimSpace(iface),
			Destination: dest,
			Gateway:     gw,
			Prefix:      prefix,
			Metric:      int(metric),
			Family:      family,
		})
	}
	return routes
}

func parseHexIPv4(hex string) (string, error) {
	if len(hex) != 8 {
		return "", fmt.Errorf("invalid length")
	}
	val, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return "", err
	}
	ip := net.IPv4(byte(val), byte(val>>8), byte(val>>16), byte(val>>24))
	return ip.String(), nil
}

func parseHexIPv6(hex string) string {
	if len(hex) != 32 {
		return ""
	}
	bytes := make([]byte, 16)
	for i := 0; i < 16; i++ {
		val, err := strconv.ParseUint(hex[i*2:i*2+2], 16, 8)
		if err != nil {
			return ""
		}
		bytes[i] = byte(val)
	}
	return net.IP(bytes).String()
}

func maskToPrefix(mask string) int {
	ip := net.ParseIP(mask)
	if ip == nil {
		return 0
	}
	ipv4 := ip.To4()
	if ipv4 == nil {
		return 0
	}
	ones, _ := net.IPv4Mask(ipv4[0], ipv4[1], ipv4[2], ipv4[3]).Size()
	return ones
}
