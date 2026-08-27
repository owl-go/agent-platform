package keycloak

import "testing"

func TestIdentityNamesAlwaysSatisfyKeycloakRequiredProfile(t *testing.T) {
	tests := []struct {
		displayName string
		first       string
		last        string
	}{
		{displayName: "Ada Lovelace", first: "Ada", last: "Lovelace"},
		{displayName: "张三", first: "张三", last: "张三"},
		{displayName: "Platform Acceptance User", first: "Platform Acceptance", last: "User"},
	}
	for _, test := range tests {
		first, last := identityNames(test.displayName)
		if first != test.first || last != test.last {
			t.Fatalf("identityNames(%q) = (%q, %q), want (%q, %q)", test.displayName, first, last, test.first, test.last)
		}
	}
}
