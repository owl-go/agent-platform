package domain

import (
	"strings"
	"testing"
	"time"
)

func TestRegisterAndChangeRuntimeImageStatus(t *testing.T) {
	now := time.Now().UTC()
	image, err := Register(Registration{
		ID: "image-1", Runtime: Claude, CLIVersion: "1.0.0", AdapterVersion: "1.0.0",
		ImageDigest:  "registry.example/claude@sha256:" + strings.Repeat("a", 64),
		Capabilities: map[string]bool{"streaming": true, "usage": true}, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if image.Status != Experimental || image.Version != 1 {
		t.Fatalf("registered image = %+v", image)
	}
	if err := image.ChangeStatus(Production, "", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := image.ChangeStatus(Blocked, "critical CVE", now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if image.Status != Blocked || image.BlockedReason != "critical CVE" || image.Version != 3 {
		t.Fatalf("updated image = %+v", image)
	}
	if err := image.ChangeStatus(Deprecated, "", now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := image.ChangeStatus(Production, "", now.Add(4*time.Minute)); err == nil {
		t.Fatal("deprecated image was reactivated")
	}
}

func TestRuntimeImageRejectsInvalidRegistration(t *testing.T) {
	base := Registration{
		ID: "image-1", Runtime: Claude, CLIVersion: "1", AdapterVersion: "1",
		ImageDigest: "registry.example/claude@sha256:" + strings.Repeat("a", 64), Now: time.Now().UTC(),
	}
	tests := []Registration{
		{Runtime: base.Runtime, CLIVersion: base.CLIVersion, AdapterVersion: base.AdapterVersion, ImageDigest: base.ImageDigest, Now: base.Now},
		{ID: base.ID, Runtime: "other", CLIVersion: base.CLIVersion, AdapterVersion: base.AdapterVersion, ImageDigest: base.ImageDigest, Now: base.Now},
		{ID: base.ID, Runtime: base.Runtime, CLIVersion: base.CLIVersion, AdapterVersion: base.AdapterVersion, ImageDigest: "latest", Now: base.Now},
		{ID: base.ID, Runtime: base.Runtime, CLIVersion: base.CLIVersion, AdapterVersion: base.AdapterVersion, ImageDigest: base.ImageDigest, Capabilities: map[string]bool{"unknown": true}, Now: base.Now},
	}
	for _, registration := range tests {
		if _, err := Register(registration); err == nil {
			t.Fatalf("Register accepted %+v", registration)
		}
	}
	image, err := Register(base)
	if err != nil {
		t.Fatal(err)
	}
	if err := image.ChangeStatus(Blocked, "", time.Now()); err == nil {
		t.Fatal("blocked status accepted no reason")
	}
}
