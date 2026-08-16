package objectstore

import (
	"encoding/hex"
	"fmt"
	"path"
	"strings"
)

func ValidateKey(key string) error {
	if key == "" || strings.HasPrefix(key, "/") || strings.HasSuffix(key, "/") || path.Clean(key) != key || key == "." || key == ".." || strings.HasPrefix(key, "../") {
		return fmt.Errorf("%w: %q", ErrInvalidKey, key)
	}
	return nil
}

func ValidatePutOptions(options PutOptions) error {
	if options.Size < 0 {
		return fmt.Errorf("object size must be non-negative")
	}
	checksum, err := hex.DecodeString(options.SHA256)
	if err != nil || len(checksum) != 32 || strings.ToLower(options.SHA256) != options.SHA256 {
		return fmt.Errorf("object SHA-256 must be 64 lowercase hexadecimal characters")
	}
	return nil
}

func NormalizePrefix(prefix string) (string, error) {
	normalized := strings.Trim(prefix, "/")
	if normalized == "" {
		return "", nil
	}
	if err := ValidateKey(normalized); err != nil {
		return "", fmt.Errorf("object key prefix: %w", err)
	}
	return normalized, nil
}
