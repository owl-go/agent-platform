package objectstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

type PreparedUpload struct {
	Path string
	File *os.File
}

func PrepareUpload(ctx context.Context, body io.Reader, options PutOptions) (*PreparedUpload, error) {
	if err := ValidatePutOptions(options); err != nil {
		return nil, err
	}
	file, err := os.CreateTemp("", "agent-object-upload-*")
	if err != nil {
		return nil, fmt.Errorf("create upload spool: %w", err)
	}
	upload := &PreparedUpload{Path: file.Name(), File: file}
	fail := func(err error) (*PreparedUpload, error) {
		_ = upload.Cleanup()
		return nil, err
	}

	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(file, hash), &contextReader{ctx: ctx, reader: io.LimitReader(body, options.Size+1)})
	if err != nil {
		return fail(fmt.Errorf("spool upload: %w", err))
	}
	if written != options.Size {
		return fail(fmt.Errorf("object size: got %d, want %d", written, options.Size))
	}
	actualChecksum := hex.EncodeToString(hash.Sum(nil))
	if actualChecksum != options.SHA256 {
		return fail(fmt.Errorf("%w: got %s, want %s", ErrChecksumMismatch, actualChecksum, options.SHA256))
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fail(fmt.Errorf("rewind upload spool: %w", err))
	}
	return upload, nil
}

func (u *PreparedUpload) Cleanup() error {
	if u == nil {
		return nil
	}
	closeErr := u.File.Close()
	removeErr := os.Remove(u.Path)
	return errorsJoin(closeErr, removeErr)
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(data []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(data)
}

func errorsJoin(errors ...error) error {
	for _, err := range errors {
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
