package processharness

import (
	"context"
	"io"
)

type streamRedactor interface {
	Reader(io.Reader) io.Reader
}

type redactingSink struct {
	redactor streamRedactor
	next     OutputSink
}

// NewRedactingSink ensures captured output is filtered before it crosses the
// persistence boundary. Output.Size continues to describe the raw byte count.
func NewRedactingSink(redactor streamRedactor, next OutputSink) OutputSink {
	return redactingSink{redactor: redactor, next: next}
}

func (s redactingSink) Store(ctx context.Context, output Output) error {
	output.Reader = s.redactor.Reader(output.Reader)
	return s.next.Store(ctx, output)
}
