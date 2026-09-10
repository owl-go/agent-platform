package openaiimages

import (
	"net/netip"
	"net/url"
	"testing"
)

func TestValidateRemoteImageURLRejectsUnsafeDestinations(t *testing.T) {
	tests := []string{
		"http://images.example.test/result.png",
		"https://127.0.0.1/result.png",
		"https://10.0.0.1/result.png",
		"https://169.254.169.254/latest/meta-data",
		"https://user:password@images.example.test/result.png",
	}
	for _, value := range tests {
		parsed, err := url.Parse(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateRemoteImageURL(parsed); err == nil {
			t.Errorf("validateRemoteImageURL(%q) unexpectedly succeeded", value)
		}
	}
}

func TestIsPublicAddress(t *testing.T) {
	if !isPublicAddress(netip.MustParseAddr("8.8.8.8")) {
		t.Error("public address was rejected")
	}
	for _, value := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "::1", "fc00::1"} {
		if isPublicAddress(netip.MustParseAddr(value)) {
			t.Errorf("private address %s was accepted", value)
		}
	}
}
