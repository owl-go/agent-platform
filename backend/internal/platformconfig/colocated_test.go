package platformconfig

import "testing"

func TestInternalOIDCTransportOnlyAcceptsALoopbackOrigin(t *testing.T) {
	config, err := Load(writeConfig(t, validYAML("postgres://database/platform")))
	if err != nil {
		t.Fatal(err)
	}
	for _, address := range []string{"http://169.254.169.254", "http://identity.internal", "http://127.0.0.1:8081/path", "http://user:password@127.0.0.1", "http://127.0.0.1?test=1"} {
		config.Authentication.InternalBaseURL = address
		if config.Authentication.Validate() == nil {
			t.Fatalf("accepted internal OIDC endpoint %q", address)
		}
	}
	config.Authentication.InternalBaseURL = "http://127.0.0.1:8081"
	if err := config.Authentication.Validate(); err != nil {
		t.Fatal(err)
	}
}
