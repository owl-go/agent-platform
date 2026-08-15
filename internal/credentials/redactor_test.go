package credentials_test

import (
	"io"
	"slices"
	"strings"
	"testing"

	"agent-platform/internal/credentials"
)

func TestRedactorRemovesKnownSecretsFromBytes(t *testing.T) {
	redactor := credentials.NewRedactor(
		[]byte("model-secret"),
		[]byte("private-key-value"),
	)

	got := string(redactor.Bytes([]byte("key=model-secret ssh=private-key-value")))
	want := "key=[REDACTED] ssh=[REDACTED]"
	if got != want {
		t.Fatalf("redacted bytes: got %q, want %q", got, want)
	}
}

type chunkReader struct {
	chunks []string
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if len(r.chunks) == 0 {
		return 0, io.EOF
	}

	chunk := r.chunks[0]
	r.chunks = r.chunks[1:]
	return copy(p, chunk), nil
}

func TestRedactorReaderHandlesSecretsAcrossReadBoundaries(t *testing.T) {
	redactor := credentials.NewRedactor([]byte("model-secret"))
	input := &chunkReader{chunks: []string{"prefix model-", "sec", "ret suffix"}}

	got, err := io.ReadAll(redactor.Reader(input))
	if err != nil {
		t.Fatalf("read redacted stream: %v", err)
	}

	want := "prefix [REDACTED] suffix"
	if string(got) != want {
		t.Fatalf("redacted stream = %q, want %q", got, want)
	}
	if strings.Contains(string(got), "model-secret") {
		t.Fatal("redacted stream contains the original secret")
	}
}

func TestRedactorHandlesBinaryPayload(t *testing.T) {
	redactor := credentials.NewRedactor([]byte{0x00, 0xff, 0x01})
	got := redactor.Bytes([]byte{0x10, 0x00, 0xff, 0x01, 0x20})
	want := append([]byte{0x10}, []byte("[REDACTED]")...)
	want = append(want, 0x20)
	if !slices.Equal(got, want) {
		t.Fatalf("redacted binary = %v, want %v", got, want)
	}
}
