// SPDX-License-Identifier: MIT

package ratelimit

import (
	"net"
	"net/http"
	"strings"
)

// DefaultTrustedCIDRs contains standard RFC 1918 private subnets and loopback CIDR blocks.
var DefaultTrustedCIDRs = []string{
	"127.0.0.1/8",    // IPv4 loopback (CIDR notation)
	"10.0.0.0/8",     // RFC 1918 class A
	"172.16.0.0/12",  // RFC 1918 class B
	"192.168.0.0/16", // RFC 1918 class C
	"::1/128",        // IPv6 loopback
	"fc00::/7",       // RFC 4193 IPv6 unique local unicast
}

// TrustedProxies represents a list of trusted proxy CIDR blocks or IP strings.
type TrustedProxies []string

// IPNets parses the TrustedProxies slice into a slice of *net.IPNet.
func (tp TrustedProxies) IPNets() []*net.IPNet {
	return ParseTrustedProxies(tp...)
}

// RealIP extracts the client IP address from the request using this TrustedProxies list.
func (tp TrustedProxies) RealIP(r *http.Request) string {
	return RealIP(r, tp.IPNets()...)
}

// RealIP extracts the client IP address from an HTTP request.
// Security rules:
//  1. Safe by default: If no trusted proxies are configured, it extracts the IP directly
//     from r.RemoteAddr and ignores all forwarding headers (X-Forwarded-For, X-Real-IP).
//  2. Untrusted peer check: If trusted proxies are configured, but the direct connection
//     peer (r.RemoteAddr) is NOT in the trusted proxy set, r.RemoteAddr IS the real client IP.
//     All forwarding headers are strictly ignored to prevent client IP spoofing.
//  3. Reverse-proxy chain traversal: If r.RemoteAddr IS a trusted proxy, it parses X-Forwarded-For
//     from right to left, stopping at the first untrusted IP (RFC 7239 reverse-proxy security model).
//     If all IPs in X-Forwarded-For are trusted, the leftmost valid IP is returned.
//  4. Fallback: If X-Forwarded-For is absent or contains only invalid IPs, X-Real-IP is checked
//     if the direct connection peer is a trusted proxy. Otherwise, r.RemoteAddr is returned.
func RealIP(r *http.Request, trusted ...*net.IPNet) string {
	if r == nil {
		return ""
	}
	remoteIP := ExtractIP(r.RemoteAddr)
	if len(trusted) == 0 {
		return remoteIP
	}

	directIP := net.ParseIP(remoteIP)
	if directIP == nil || !IsTrusted(directIP, trusted) {
		return remoteIP
	}

	// Direct connection is trusted: parse X-Forwarded-For right-to-left.
	if ip := parseXFF(r, trusted); ip != "" {
		return ip
	}

	// Fall back to X-Real-IP if present and direct connection is trusted.
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		host := ExtractIP(trimSpace(xri))
		if ip := net.ParseIP(host); ip != nil {
			return ip.String()
		}
	}

	return remoteIP
}

func parseXFF(r *http.Request, trusted []*net.IPNet) string {
	var xffParts []string
	for _, h := range r.Header.Values("X-Forwarded-For") {
		for _, part := range strings.Split(h, ",") {
			if trimmed := trimSpace(part); trimmed != "" {
				xffParts = append(xffParts, trimmed)
			}
		}
	}

	var leftmostValid string
	for i := len(xffParts) - 1; i >= 0; i-- {
		host := ExtractIP(xffParts[i])
		ip := net.ParseIP(host)
		if ip == nil {
			continue
		}
		leftmostValid = ip.String()
		if !IsTrusted(ip, trusted) {
			return ip.String()
		}
	}
	return leftmostValid
}

// RealIPWithProxies is a convenience helper that parses CIDR/IP strings and extracts the real IP.
func RealIPWithProxies(r *http.Request, trustedCIDRs ...string) string {
	return RealIP(r, ParseTrustedProxies(trustedCIDRs...)...)
}

// IsTrusted checks if an IP matches any of the trusted IPNet ranges.
func IsTrusted(ip net.IP, trusted []*net.IPNet) bool {
	for _, n := range trusted {
		if n != nil && n.Contains(ip) {
			return true
		}
	}
	return false
}

// ExtractIP parses the host IP portion from a RemoteAddr (e.g. "127.0.0.1:8080" -> "127.0.0.1", "[::1]:80" -> "::1").
func ExtractIP(remoteAddr string) string {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	host = strings.Trim(host, "[]")
	return host
}

// ParseCIDRorIP parses a string as either a CIDR (e.g. "10.0.0.0/8", "127.0.0.1/8") or a single IP (e.g. "127.0.0.1").
func ParseCIDRorIP(s string) (*net.IPNet, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, &net.ParseError{Type: "IP address or CIDR", Text: s}
	}
	if strings.Contains(s, "/") {
		_, ipNet, err := net.ParseCIDR(s)
		return ipNet, err
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return nil, &net.ParseError{Type: "IP address", Text: s}
	}
	if ip.To4() != nil {
		_, ipNet, err := net.ParseCIDR(s + "/32")
		return ipNet, err
	}
	_, ipNet, err := net.ParseCIDR(s + "/128")
	return ipNet, err
}

// ParseTrustedProxies parses multiple CIDRs or IP strings (including comma-separated lists).
func ParseTrustedProxies(cidrs ...string) []*net.IPNet {
	var res []*net.IPNet
	for _, raw := range cidrs {
		for _, c := range strings.Split(raw, ",") {
			c = strings.TrimSpace(c)
			if c == "" {
				continue
			}
			if ipNet, err := ParseCIDRorIP(c); err == nil && ipNet != nil {
				res = append(res, ipNet)
			}
		}
	}
	return res
}

// KeyFuncWithTrustedProxies returns a KeyFunc that extracts the real client IP
// honoring the specified trusted proxy CIDRs.
func KeyFuncWithTrustedProxies(cidrs ...string) KeyFunc {
	trusted := ParseTrustedProxies(cidrs...)
	return func(r *http.Request) string {
		return RealIP(r, trusted...)
	}
}

// trimSpace is a minimal whitespace trimmer.
func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r' || s[start] == '\n') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}
