package webhook

import (
	"fmt"
	"net"
	"net/url"
)

func ValidateURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("url is required")
	}
	if len(rawURL) > 2048 {
		return fmt.Errorf("url exceeds maximum length of 2048 characters")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}

	host := u.Hostname()

	switch u.Scheme {
	case "https":
		// https is allowed, but check for private IPs
	case "http":
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			return fmt.Errorf("http is only allowed for localhost")
		}
		return nil
	default:
		return fmt.Errorf("url scheme must be https (or http for localhost)")
	}

	ip := net.ParseIP(host)
	if ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("url must not point to a private IP address")
		}
	} else {
		addrs, err := net.LookupHost(host)
		if err == nil {
			for _, addr := range addrs {
				if pip := net.ParseIP(addr); pip != nil && isPrivateIP(pip) {
					return fmt.Errorf("url must not resolve to a private IP address")
				}
			}
		}
	}

	return nil
}

func isPrivateIP(ip net.IP) bool {
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"127.0.0.0/8",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
