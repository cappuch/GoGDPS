package netutil

import (
	"net"
	"strings"
)

// cloudflareRanges mirrors mainLib::isCloudFlareIP ranges from incl/lib/mainLib.php.
var cloudflareRanges = []string{
	"173.245.48.0/20",
	"103.21.244.0/22",
	"103.22.200.0/22",
	"103.31.4.0/22",
	"141.101.64.0/18",
	"108.162.192.0/18",
	"190.93.240.0/20",
	"188.114.96.0/20",
	"197.234.240.0/22",
	"198.41.128.0/17",
	"162.158.0.0/15",
	"104.16.0.0/13",
	"104.24.0.0/14",
	"172.64.0.0/13",
	"131.0.72.0/22",
}

// IPv4InRange checks if ip is within range (CIDR, wildcard, or start-end).
// Port of ipInRange::ipv4_in_range from incl/lib/ip_in_range.php.
func IPv4InRange(ip, rng string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() == nil {
		return false
	}
	ip4 := parsed.To4()

	if strings.Contains(rng, "/") {
		parts := strings.SplitN(rng, "/", 2)
		base := parts[0]
		mask := parts[1]

		if strings.Contains(mask, ".") {
			maskIP := net.ParseIP(strings.ReplaceAll(mask, "*", "0"))
			if maskIP == nil {
				return false
			}
			m := maskIP.To4()
			for i := 0; i < 4; i++ {
				if ip4[i]&m[i] != net.ParseIP(base).To4()[i]&m[i] {
					return false
				}
			}
			return true
		}

		_, cidr, err := net.ParseCIDR(normalizeCIDR(base, mask))
		if err != nil {
			return false
		}
		return cidr.Contains(parsed)
	}

	if strings.Contains(rng, "*") {
		lower := strings.ReplaceAll(rng, "*", "0")
		upper := strings.ReplaceAll(rng, "*", "255")
		rng = lower + "-" + upper
	}

	if strings.Contains(rng, "-") {
		parts := strings.SplitN(rng, "-", 2)
		low := net.ParseIP(strings.TrimSpace(parts[0]))
		high := net.ParseIP(strings.TrimSpace(parts[1]))
		if low == nil || high == nil {
			return false
		}
		ipNum := ipToUint32(ip4)
		return ipNum >= ipToUint32(low.To4()) && ipNum <= ipToUint32(high.To4())
	}
	return false
}

func normalizeCIDR(base, maskBits string) string {
	parts := strings.Split(base, ".")
	for len(parts) < 4 {
		parts = append(parts, "0")
	}
	for i, p := range parts {
		if p == "" {
			parts[i] = "0"
		}
	}
	return strings.Join(parts, ".") + "/" + maskBits
}

func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func isCloudflareIP(ip string) bool {
	for _, rng := range cloudflareRanges {
		if IPv4InRange(ip, rng) {
			return true
		}
	}
	return false
}
