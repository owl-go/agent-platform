package secretcrypto

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestBoxRoundTripAndContextBinding(t *testing.T) {
	box, err := New(base64.RawStdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Encrypt([]byte("secret"), "model:1")
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := box.Decrypt(ciphertext, "model:1")
	if err != nil || string(plaintext) != "secret" {
		t.Fatalf("round trip = %q, %v", plaintext, err)
	}
	if _, err := box.Decrypt(ciphertext, "model:2"); err == nil {
		t.Fatal("expected associated context mismatch to fail")
	}
}
