package objectstore

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

func EncodeMetadata(metadata map[string]string) (string, error) {
	if metadata == nil {
		metadata = map[string]string{}
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("encode object metadata: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func DecodeMetadata(encoded string) (map[string]string, error) {
	if encoded == "" {
		return map[string]string{}, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode object metadata: %w", err)
	}
	metadata := make(map[string]string)
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("decode object metadata JSON: %w", err)
	}
	return metadata, nil
}
