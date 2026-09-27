package domain

import (
	"testing"
	"time"
)

func TestConnectorPublicationTransitionsPreserveSource(t *testing.T) {
	now := time.Now().UTC()
	publication := ConnectorPublication{PackageSource: "feishu", Version: 0}
	if err := publication.Publish("admin-1", "revision-1", now); err != nil {
		t.Fatal(err)
	}
	if publication.State != ConnectorPublicationAvailable || publication.ActiveRevisionID != "revision-1" || publication.AdministratorID != "admin-1" || publication.Version != 1 || !publication.UpdatedAt.Equal(now) {
		t.Fatalf("unexpected published projection: %#v", publication)
	}
	if err := publication.Disable("admin-2", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if publication.State != ConnectorPublicationDisabled || publication.PackageSource != "feishu" || publication.ActiveRevisionID != "revision-1" || publication.AdministratorID != "admin-2" || publication.Version != 2 {
		t.Fatalf("unexpected disabled projection: %#v", publication)
	}
}

func TestConnectorInstallationUpgradeFailureKeepsActiveRevisionAndAuthorization(t *testing.T) {
	now := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	installation := ConnectorInstallation{ID: "install-1", OwnerID: "user-1", PackageSource: "example", ActiveRevisionID: "rev-1", AuthorizationID: "auth-1", State: ConnectorInstallationActive}
	if err := installation.ActivateRevision("user-1", "rev-2", now); err != nil {
		t.Fatal(err)
	}
	if installation.ActiveRevisionID != "rev-2" || installation.State != ConnectorInstallationActive {
		t.Fatalf("unexpected activation: %#v", installation)
	}
	if err := installation.RollbackRevision("user-1", "rev-1", "upgrade validation failed", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if installation.ActiveRevisionID != "rev-1" || installation.AuthorizationID != "auth-1" || installation.State != ConnectorInstallationActive {
		t.Fatalf("rollback lost active state: %#v", installation)
	}
}

func TestConnectorAuthorizationDisconnectFailsClosed(t *testing.T) {
	authorization := ConnectorAuthorization{ID: "auth-1", OwnerID: "user-1", InstallationID: "install-1", State: ConnectorAuthorizationActive}
	if err := authorization.Disconnect("user-1"); err != nil {
		t.Fatal(err)
	}
	if authorization.State != ConnectorAuthorizationDisconnected {
		t.Fatalf("state = %q", authorization.State)
	}
	if authorization.CanInvoke() {
		t.Fatal("disconnected authorization can invoke")
	}
}
