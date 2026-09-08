package credentials

import (
	"bytes"
	"io"
	"sort"
	"sync"
)

const redactedValue = "[REDACTED]"

// Redactor replaces exact credential values without interpreting them as patterns.
type Redactor struct {
	mu       sync.RWMutex
	patterns [][]byte
}

func NewRedactor(patterns ...[]byte) *Redactor {
	copied := make([][]byte, 0, len(patterns))
	for _, pattern := range patterns {
		if len(pattern) == 0 {
			continue
		}

		copied = append(copied, bytes.Clone(pattern))
	}

	sort.SliceStable(copied, func(i, j int) bool {
		return len(copied[i]) > len(copied[j])
	})

	return &Redactor{patterns: copied}
}

func (r *Redactor) Bytes(input []byte) []byte {
	output := bytes.Clone(input)
	if r == nil {
		return output
	}
	patterns := r.Patterns()
	for _, pattern := range patterns {
		output = bytes.ReplaceAll(output, pattern, []byte(redactedValue))
	}

	return output
}

func (r *Redactor) Reader(source io.Reader) io.Reader {
	patterns := r.Patterns()
	if len(patterns) == 0 {
		return source
	}
	return &redactingReader{source: source, patterns: patterns}
}

// AddPatterns registers credentials discovered lazily during an operation.
func (r *Redactor) AddPatterns(patterns ...[]byte) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, pattern := range patterns {
		if len(pattern) > 0 {
			r.patterns = append(r.patterns, bytes.Clone(pattern))
		}
	}
	sort.SliceStable(r.patterns, func(i, j int) bool { return len(r.patterns[i]) > len(r.patterns[j]) })
}

func (r *Redactor) Patterns() [][]byte {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	patterns := make([][]byte, 0, len(r.patterns))
	for _, pattern := range r.patterns {
		patterns = append(patterns, bytes.Clone(pattern))
	}
	return patterns
}

type redactingReader struct {
	source   io.Reader
	patterns [][]byte
	pending  []byte
	output   []byte
	err      error
}

func (r *redactingReader) Read(destination []byte) (int, error) {
	for len(r.output) == 0 && r.err == nil {
		chunk := make([]byte, 32*1024)
		count, err := r.source.Read(chunk)
		if count > 0 {
			r.pending = append(r.pending, chunk[:count]...)
		}
		if err != nil {
			r.err = err
		}

		r.flushPending(err != nil)
	}

	if len(r.output) > 0 {
		count := copy(destination, r.output)
		r.output = r.output[count:]
		return count, nil
	}

	return 0, r.err
}

func (r *redactingReader) flushPending(final bool) {
	for len(r.pending) > 0 {
		matched := false
		couldMatch := false
		for _, pattern := range r.patterns {
			if bytes.HasPrefix(r.pending, pattern) {
				r.output = append(r.output, redactedValue...)
				r.pending = r.pending[len(pattern):]
				matched = true
				break
			}
			if bytes.HasPrefix(pattern, r.pending) {
				couldMatch = true
			}
		}

		if matched {
			continue
		}
		if couldMatch && !final {
			return
		}

		r.output = append(r.output, r.pending[0])
		r.pending = r.pending[1:]
	}
}
