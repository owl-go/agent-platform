package sandbox

import (
	"strings"
	"testing"
)

func TestExecutionProfileRejectsImplicitDowngradeAndMutableImages(t *testing.T) {
	for _, test := range []struct {
		profile, runtime string
		valid            bool
	}{
		{"", "runsc", true}, {"production", "runsc", true},
		{"", "runc", false}, {"production", "runc", false},
		{"local", "runc", true}, {"local", "runsc", false}, {"other", "runc", false},
	} {
		if err := ValidateExecutionProfile(test.profile, test.runtime); (err == nil) != test.valid {
			t.Fatalf("profile %q runtime %q: %v", test.profile, test.runtime, err)
		}
	}
	for _, profile := range []string{"", "production", "local"} {
		if ValidExecutionImage(profile, "runtime:latest") || ValidExecutionImage(profile, "sha256:short") {
			t.Fatal("mutable or malformed image accepted")
		}
		if !ValidExecutionImage(profile, "registry.example/runtime@sha256:"+strings.Repeat("a", 64)) {
			t.Fatal("Registry RepoDigest rejected")
		}
		if ValidExecutionImage(profile, "sha256:"+strings.Repeat("a", 64)) != (profile == LocalProfile) {
			t.Fatal("local Image ID escaped explicit local profile")
		}
	}
}
