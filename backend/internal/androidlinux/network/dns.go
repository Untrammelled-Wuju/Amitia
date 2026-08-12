package network

import (
	"context"
	"net"
	"slices"
	"strings"
)

func performDNSLookup(ctx context.Context, host, queryType string, maxResults int) (DNSLookupResult, error) {
	result := DNSLookupResult{
		Host: host,
		Type: strings.ToUpper(queryType),
	}

	if result.Type == "" {
		result.Type = "IP"
	}

	switch result.Type {
	case "A", "IP":
		ips, err := lookupIP(ctx, host)
		if err != nil {
			return DNSLookupResult{}, err
		}
		for _, ip := range ips {
			if ip.To4() != nil {
				result.Addresses = append(result.Addresses, ip.String())
			}
		}
	case "AAAA":
		ips, err := lookupIP(ctx, host)
		if err != nil {
			return DNSLookupResult{}, err
		}
		for _, ip := range ips {
			if ip.To4() == nil {
				result.Addresses = append(result.Addresses, ip.String())
			}
		}
	case "CNAME":
		cname, err := net.DefaultResolver.LookupCNAME(ctx, host)
		if err != nil {
			return DNSLookupResult{}, err
		}
		result.CNAME = strings.TrimSuffix(cname, ".")
	case "TXT":
		txts, err := net.DefaultResolver.LookupTXT(ctx, host)
		if err != nil {
			return DNSLookupResult{}, err
		}
		result.TXT = txts
	case "MX":
		mxs, err := net.DefaultResolver.LookupMX(ctx, host)
		if err != nil {
			return DNSLookupResult{}, err
		}
		result.MX = make([]MX, 0, len(mxs))
		for _, mx := range mxs {
			result.MX = append(result.MX, MX{Host: strings.TrimSuffix(mx.Host, "."), Pref: mx.Pref})
		}
	default:
		ips, err := lookupIP(ctx, host)
		if err != nil {
			return DNSLookupResult{}, err
		}
		for _, ip := range ips {
			result.Addresses = append(result.Addresses, ip.String())
		}
	}

	if maxResults > 0 && len(result.Addresses) > maxResults {
		result.Addresses = result.Addresses[:maxResults]
	}
	if maxResults > 0 && len(result.TXT) > maxResults {
		result.TXT = result.TXT[:maxResults]
	}
	if maxResults > 0 && len(result.MX) > maxResults {
		result.MX = result.MX[:maxResults]
	}

	slices.Sort(result.Addresses)
	slices.Sort(result.TXT)
	slices.SortFunc(result.MX, func(a, b MX) int {
		return strings.Compare(a.Host, b.Host)
	})

	return result, nil
}

func lookupIP(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	return ips, nil
}
