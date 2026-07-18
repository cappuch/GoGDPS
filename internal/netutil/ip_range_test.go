package netutil

import "testing"

func TestIPv4InRangeCIDR(t *testing.T) {
	if !IPv4InRange("173.245.48.1", "173.245.48.0/20") {
		t.Fatal("expected cloudflare range match")
	}
	if IPv4InRange("8.8.8.8", "173.245.48.0/20") {
		t.Fatal("expected no match")
	}
}

func TestIPv4InRangeLocalhost(t *testing.T) {
	if !IPv4InRange("127.0.0.1", "127.0.0.0/8") {
		t.Fatal("expected localhost in range")
	}
}
